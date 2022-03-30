package api

import (
	"encoding/hex"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/idena-network/idena-go/blockchain"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-indexer-api/app/monitoring"
	service2 "github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/idena-network/idena-indexer-api/config"
	"github.com/idena-network/idena-indexer-api/docs"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	httpSwagger "github.com/swaggo/http-swagger"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server interface {
	Start(swaggerConfig config.SwaggerConfig)
}

func NewServer(
	port int,
	latestHours int,
	activeAddrHours int,
	frozenBalanceAddrs []string,
	getDumpLink func() string,
	service Service,
	contractsService service2.Contracts,
	logger log.Logger,
	pm monitoring.PerformanceMonitor,
	maxReqCount int,
	timeout time.Duration,
	reqsPerMinuteLimit int,
	dynamicEndpointLoader service2.DynamicEndpointLoader,
	cors bool,
) Server {
	var lowerFrozenBalanceAddrs []string
	for _, frozenBalanceAddr := range frozenBalanceAddrs {
		lowerFrozenBalanceAddrs = append(lowerFrozenBalanceAddrs, strings.ToLower(frozenBalanceAddr))
	}
	return &httpServer{
		port:               port,
		service:            service,
		contractsService:   contractsService,
		logger:             logger,
		latestHours:        latestHours,
		activeAddrHours:    activeAddrHours,
		frozenBalanceAddrs: lowerFrozenBalanceAddrs,
		getDumpLink:        getDumpLink,
		pm:                 pm,
		cors:               cors,
		limiter: &reqLimiter{
			queue:               make(chan struct{}, maxReqCount),
			adjacentDataQueue:   make(chan struct{}, 1),
			timeout:             timeout,
			reqCountsByClientId: cache.New(time.Second*30, time.Minute*5),
			reqLimit:            reqsPerMinuteLimit / 2,
		},
		dynamicEndpointLoader: dynamicEndpointLoader,
	}
}

type httpServer struct {
	port               int
	latestHours        int
	activeAddrHours    int
	frozenBalanceAddrs []string
	service            Service
	contractsService   service2.Contracts
	limiter            *reqLimiter
	logger             log.Logger
	pm                 monitoring.PerformanceMonitor
	counter            int
	mutex              sync.Mutex
	getDumpLink        func() string
	cors               bool

	dynamicEndpointLoader    service2.DynamicEndpointLoader
	dynamicEndpointsHash     string
	dynamicEndpointsByMethod map[string]types.DynamicEndpoint
}

func (s *httpServer) generateReqId() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	id := s.counter
	s.counter++
	return id
}

func (s *httpServer) Start(swaggerConfig config.SwaggerConfig) {
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api").Subrouter()
	s.initRouter(apiRouter)
	if swaggerConfig.Enabled {
		docs.SwaggerInfo.Title = "Idena API"
		docs.SwaggerInfo.Version = "0.1.0"
		docs.SwaggerInfo.Host = swaggerConfig.Host
		docs.SwaggerInfo.BasePath = swaggerConfig.BasePath
		apiRouter.PathPrefix("/swagger").Handler(httpSwagger.Handler(
			httpSwagger.URL("/api/swagger/doc.json"),
		))
	}
	handler := s.requestFilter(apiRouter)
	if s.cors {
		headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type"})
		originsOk := handlers.AllowedOrigins([]string{"*"})
		methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})
		handler = handlers.CORS(originsOk, headersOk, methodsOk)(handler)
	}
	err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), handler)
	if err != nil {
		panic(err)
	}
}

func (s *httpServer) refreshDynamicEndpoints() {
	dynamicEndpoints, err := s.dynamicEndpointLoader.Load()
	if err != nil {
		s.logger.Error(fmt.Sprintf("Unable to load dynamic endpoints: %v", err.Error()))
		return
	}
	dynamicEndpointsHashArr := make([]string, 0, len(dynamicEndpoints)*3)
	for _, dynamicEndpoint := range dynamicEndpoints {
		var limit int
		if dynamicEndpoint.Limit != nil {
			limit = *dynamicEndpoint.Limit
		}
		dynamicEndpointsHashArr = append(dynamicEndpointsHashArr, dynamicEndpoint.DataSource, dynamicEndpoint.Method, strconv.Itoa(limit))
	}
	dynamicEndpointsHash := strings.Join(dynamicEndpointsHashArr, "")
	if s.dynamicEndpointsHash == dynamicEndpointsHash {
		return
	}
	dynamicEndpointsByMethod := make(map[string]types.DynamicEndpoint, len(dynamicEndpoints))
	for _, dynamicEndpoint := range dynamicEndpoints {
		dynamicEndpointsByMethod[strings.ToLower(dynamicEndpoint.Method)] = dynamicEndpoint
	}
	s.dynamicEndpointsHash = dynamicEndpointsHash
	s.dynamicEndpointsByMethod = dynamicEndpointsByMethod
	s.logger.Info("Dynamic endpoints updated")
	return
}

func (s *httpServer) requestFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := s.generateReqId()
		var urlToLog *url.URL
		lowerUrlPath := strings.ToLower(r.URL.Path)
		if !strings.Contains(lowerUrlPath, "/search") {
			urlToLog = r.URL
		}
		ip := GetIP(r)
		s.logger.Debug("Got api request", "reqId", reqId, "url", urlToLog, "from", ip)
		if err := s.limiter.takeResource(ip, lowerUrlPath); err != nil {
			s.logger.Error("Unable to handle API request", "reqId", reqId, "err", err)
			switch err {
			case errTimeout:
				w.WriteHeader(http.StatusServiceUnavailable)
				break
			case errReqLimitExceed:
				w.WriteHeader(http.StatusTooManyRequests)
				break
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
			WriteErrorResponse(w, err, s.logger)
			return
		}
		defer s.limiter.releaseResource(lowerUrlPath)

		err := r.ParseForm()
		if err != nil {
			s.logger.Error("Unable to parse API request", "reqId", reqId, "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer s.logger.Debug("Completed api request", "reqId", reqId)
		for name, value := range r.Form {
			r.Form[strings.ToLower(name)] = value
		}
		r.URL.Path = strings.ToLower(r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *httpServer) initRouter(router *mux.Router) {
	router.Path(strings.ToLower("/DumpLink")).HandlerFunc(s.dumpLink)

	router.Path(strings.ToLower("/Search")).
		Queries("value", "{value}").
		HandlerFunc(s.search)

	router.Path(strings.ToLower("/Coins")).
		HandlerFunc(s.coins)
	router.Path(strings.ToLower("/Txt/TotalSupply")).
		HandlerFunc(s.txtTotalSupply)

	router.Path(strings.ToLower("/CirculatingSupply")).
		Queries("format", "{format}").
		HandlerFunc(s.circulatingSupply)
	router.Path(strings.ToLower("/CirculatingSupply")).
		HandlerFunc(s.circulatingSupply)

	router.Path(strings.ToLower("/Txt/CirculatingSupply")).
		HandlerFunc(s.txtCirculatingSupply)

	router.Path(strings.ToLower("/Upgrades")).
		HandlerFunc(s.upgrades)

	router.Path(strings.ToLower("/Epochs/Count")).HandlerFunc(s.epochsCount)
	router.Path(strings.ToLower("/Epochs")).HandlerFunc(s.epochs)

	router.Path(strings.ToLower("/Epoch/Last")).HandlerFunc(s.lastEpoch)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}")).HandlerFunc(s.epoch)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Blocks/Count")).
		HandlerFunc(s.epochBlocksCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Blocks")).
		HandlerFunc(s.epochBlocks)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Flips/Count")).
		HandlerFunc(s.epochFlipsCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Flips")).
		HandlerFunc(s.epochFlips)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/FlipStatesSummary")).HandlerFunc(s.epochFlipStatesSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/FlipWrongWordsSummary")).HandlerFunc(s.epochFlipWrongWordsSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identities/Count")).
		HandlerFunc(s.epochIdentitiesCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identities")).
		HandlerFunc(s.epochIdentities)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/IdentityStatesSummary")).HandlerFunc(s.epochIdentityStatesSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/IdentityStatesInterimSummary")).HandlerFunc(s.epochIdentityStatesInterimSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/InvitesSummary")).HandlerFunc(s.epochInvitesSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/InviteStatesSummary")).HandlerFunc(s.epochInviteStatesSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Invites/Count")).
		HandlerFunc(s.epochInvitesCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Invites")).
		HandlerFunc(s.epochInvites)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Txs/Count")).
		HandlerFunc(s.epochTxsCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Txs")).
		HandlerFunc(s.epochTxs)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Coins")).
		HandlerFunc(s.epochCoins)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/RewardsSummary")).HandlerFunc(s.epochRewardsSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Authors/Bad/Count")).HandlerFunc(s.epochBadAuthorsCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Authors/Bad")).
		HandlerFunc(s.epochBadAuthors)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/IdentityRewards/Count")).HandlerFunc(s.epochIdentitiesRewardsCount)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/IdentityRewards")).
		HandlerFunc(s.epochIdentitiesRewards)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/FundPayments")).HandlerFunc(s.epochFundPayments)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/RewardBounds")).HandlerFunc(s.epochRewardBounds)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/DelegateeTotalRewards")).HandlerFunc(s.epochDelegateeTotalRewards)

	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}")).HandlerFunc(s.epochIdentity)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/FlipsToSolve/Short")).
		HandlerFunc(s.epochIdentityShortFlipsToSolve)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/FlipsToSolve/Long")).
		HandlerFunc(s.epochIdentityLongFlipsToSolve)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/Answers/Short")).
		HandlerFunc(s.epochIdentityShortAnswers)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/Answers/Long")).
		HandlerFunc(s.epochIdentityLongAnswers)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/Flips")).HandlerFunc(s.epochIdentityFlips)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/Rewards")).
		HandlerFunc(s.epochIdentityRewards)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/RewardedFlips")).
		HandlerFunc(s.epochIdentityFlipsWithRewardFlag)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/ReportRewards")).
		HandlerFunc(s.epochIdentityReportedFlipRewards)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/Authors/Bad")).
		HandlerFunc(s.epochIdentityBadAuthor)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/RewardedInvites")).
		HandlerFunc(s.epochIdentityInvitesWithRewardFlag)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/SavedInviteRewards")).
		HandlerFunc(s.epochIdentitySavedInviteRewards)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/AvailableInvites")).
		HandlerFunc(s.epochIdentityAvailableInvites)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/ValidationSummary")).
		HandlerFunc(s.epochIdentityValidationSummary)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Identity/{address}/DataWithProof")).
		HandlerFunc(s.epochIdentityDataWithProof)

	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Address/{address}/DelegateeRewards")).HandlerFunc(s.epochDelegateeRewards)
	router.Path(strings.ToLower("/Epoch/{epoch:[0-9]+}/Address/{address}/DelegateeTotalRewards")).HandlerFunc(s.epochAddressDelegateeTotalRewards)

	router.Path(strings.ToLower("/Block/Last")).HandlerFunc(s.lastBlock)
	router.Path(strings.ToLower("/Block/{id}")).HandlerFunc(s.block)
	router.Path(strings.ToLower("/Block/{id}/Txs/Count")).HandlerFunc(s.blockTxsCount)
	router.Path(strings.ToLower("/Block/{id}/Txs")).HandlerFunc(s.blockTxs)
	router.Path(strings.ToLower("/Block/{id}/Coins")).HandlerFunc(s.blockCoins)

	router.Path(strings.ToLower("/Identity/{address}")).HandlerFunc(s.identity)
	router.Path(strings.ToLower("/Identity/{address}/Age")).HandlerFunc(s.identityAge)
	router.Path(strings.ToLower("/Identity/{address}/CurrentFlipCids")).HandlerFunc(s.identityCurrentFlipCids)
	router.Path(strings.ToLower("/Identity/{address}/Epochs/Count")).HandlerFunc(s.identityEpochsCount)
	router.Path(strings.ToLower("/Identity/{address}/Epochs")).
		HandlerFunc(s.identityEpochs)
	router.Path(strings.ToLower("/Identity/{address}/Flips/Count")).HandlerFunc(s.identityFlipsCount)
	router.Path(strings.ToLower("/Identity/{address}/Flips")).HandlerFunc(s.identityFlips)
	router.Path(strings.ToLower("/Identity/{address}/FlipStates")).HandlerFunc(s.identityFlipStates)
	router.Path(strings.ToLower("/Identity/{address}/FlipQualifiedAnswers")).HandlerFunc(s.identityFlipRightAnswers)
	router.Path(strings.ToLower("/Identity/{address}/Invites/Count")).HandlerFunc(s.identityInvitesCount)
	router.Path(strings.ToLower("/Identity/{address}/Invites")).
		HandlerFunc(s.identityInvites)
	router.Path(strings.ToLower("/Identity/{address}/Rewards/Count")).HandlerFunc(s.identityRewardsCount)
	router.Path(strings.ToLower("/Identity/{address}/Rewards")).
		HandlerFunc(s.identityRewards)
	router.Path(strings.ToLower("/Identity/{address}/EpochRewards/Count")).HandlerFunc(s.identityEpochRewardsCount)
	router.Path(strings.ToLower("/Identity/{address}/EpochRewards")).
		HandlerFunc(s.identityEpochRewards)
	router.Path(strings.ToLower("/Identity/{address}/Authors/Bad/Count")).HandlerFunc(s.addressBadAuthorsCount)
	router.Path(strings.ToLower("/Identity/{address}/Authors/Bad")).HandlerFunc(s.addressBadAuthors)

	router.Path(strings.ToLower("/Flip/{hash}")).HandlerFunc(s.flip)
	router.Path(strings.ToLower("/Flip/{hash}/Content")).HandlerFunc(s.flipContent)
	router.Path(strings.ToLower("/Flip/{hash}/Answers/Short")).
		HandlerFunc(s.flipShortAnswers)
	router.Path(strings.ToLower("/Flip/{hash}/Answers/Long")).
		HandlerFunc(s.flipLongAnswers)
	router.Path(strings.ToLower("/Flip/{hash}/Epoch/AdjacentFlips")).HandlerFunc(s.flipEpochAdjacentFlips)

	router.Path(strings.ToLower("/Transaction/{hash}")).HandlerFunc(s.transaction)
	router.Path(strings.ToLower("/Transaction/{hash}/Raw")).HandlerFunc(s.transactionRaw)

	router.Path(strings.ToLower("/Address/{address}")).HandlerFunc(s.address)
	router.Path(strings.ToLower("/Address/{address}/Txs/Count")).HandlerFunc(s.identityTxsCount)
	router.Path(strings.ToLower("/Address/{address}/Txs")).
		HandlerFunc(s.identityTxs)
	router.Path(strings.ToLower("/Address/{address}/Penalties/Count")).HandlerFunc(s.addressPenaltiesCount)
	router.Path(strings.ToLower("/Address/{address}/Penalties")).HandlerFunc(s.addressPenalties)

	// Deprecated path
	router.Path(strings.ToLower("/Address/{address}/Flips/Count")).HandlerFunc(s.identityFlipsCount)
	// Deprecated path
	router.Path(strings.ToLower("/Address/{address}/Flips")).HandlerFunc(s.identityFlips)

	router.Path(strings.ToLower("/Address/{address}/States/Count")).HandlerFunc(s.addressStatesCount)
	router.Path(strings.ToLower("/Address/{address}/States")).HandlerFunc(s.addressStates)
	router.Path(strings.ToLower("/Address/{address}/TotalLatestMiningReward")).
		HandlerFunc(s.addressTotalLatestMiningReward)
	router.Path(strings.ToLower("/Address/{address}/TotalLatestBurntCoins")).
		HandlerFunc(s.addressTotalLatestBurntCoins)

	router.Path(strings.ToLower("/Address/{address}/Balance/Changes")).HandlerFunc(s.addressBalanceUpdates)
	router.Path(strings.ToLower("/Address/{address}/Balance/Changes/Summary")).HandlerFunc(s.addressBalanceUpdatesSummary)

	router.Path(strings.ToLower("/Address/{address}/DelegateeTotalRewards")).HandlerFunc(s.addressDelegateeTotalRewards)

	router.Path(strings.ToLower("/Balances")).HandlerFunc(s.balances)
	router.Path(strings.ToLower("/Staking")).HandlerFunc(s.staking)

	router.Path(strings.ToLower("/Contract/{address}")).HandlerFunc(s.contract)
	router.Path(strings.ToLower("/Contract/{address}/BalanceUpdates")).HandlerFunc(s.contractTxBalanceUpdates)

	router.Path(strings.ToLower("/TimeLockContract/{address}")).HandlerFunc(s.timeLockContract)
	router.Path(strings.ToLower("/OracleLockContract/{address}")).HandlerFunc(s.oracleLockContract)
	router.Path(strings.ToLower("/MultisigContract/{address}")).HandlerFunc(s.multisigContract)

	router.Path(strings.ToLower("/OracleVotingContracts")).HandlerFunc(s.oracleVotingContracts)
	router.Path(strings.ToLower("/OracleVotingContract/{address}")).HandlerFunc(s.oracleVotingContract)
	router.Path(strings.ToLower("/Address/{address}/OracleVotingContracts")).HandlerFunc(s.addressOracleVotingContracts)
	router.Path(strings.ToLower("/Address/{address}/Contract/{contractAddress}/BalanceUpdates")).HandlerFunc(s.addressContractTxBalanceUpdates)
	router.Path(strings.ToLower("/OracleVotingContracts/EstimatedOracleRewards")).HandlerFunc(s.estimatedOracleRewards)

	router.Path(strings.ToLower("/MemPool/Txs")).HandlerFunc(s.memPoolTxs)
	router.Path(strings.ToLower("/MemPool/Txs/Count")).HandlerFunc(s.memPoolTxsCount)

	router.Path(strings.ToLower("/OnlineIdentities/Count")).HandlerFunc(s.onlineIdentitiesCount)
	router.Path(strings.ToLower("/OnlineIdentities")).
		HandlerFunc(s.onlineIdentities)

	router.Path(strings.ToLower("/OnlineIdentity/{address}")).HandlerFunc(s.onlineIdentity)

	router.Path(strings.ToLower("/OnlineMiners/Count")).HandlerFunc(s.onlineCount)
	router.Path(strings.ToLower("/Miners/History")).HandlerFunc(s.minersHistory)
	router.Path(strings.ToLower("/Peers/History")).HandlerFunc(s.peersHistory)

	router.Path(strings.ToLower("/Validators/Count")).HandlerFunc(s.validatorsCount)
	router.Path(strings.ToLower("/Validators")).HandlerFunc(s.validators)
	router.Path(strings.ToLower("/OnlineValidators/Count")).HandlerFunc(s.onlineValidatorsCount)
	router.Path(strings.ToLower("/OnlineValidators")).HandlerFunc(s.onlineValidators)

	router.Path(strings.ToLower("/SignatureAddress")).
		Queries("value", "{value}", "signature", "{signature}").
		HandlerFunc(s.signatureAddress)

	router.Path(strings.ToLower("/UpgradeVotings")).HandlerFunc(s.upgradeVotings)
	router.Path(strings.ToLower("/UpgradeVoting")).HandlerFunc(s.upgradeVoting)
	router.Path(strings.ToLower("/Upgrade/{upgrade:[0-9]+}/VotingHistory")).HandlerFunc(s.upgradeVotingHistory)
	router.Path(strings.ToLower("/Upgrade/{upgrade:[0-9]+}")).HandlerFunc(s.upgrade)
	router.Path(strings.ToLower("/Node/{version}/ForkChangeLog")).HandlerFunc(s.forkChangeLog)

	router.Path(strings.ToLower("/Now")).HandlerFunc(s.now)

	router.Path(strings.ToLower("/Pools/Count")).HandlerFunc(s.poolsCount)
	router.Path(strings.ToLower("/Pools")).HandlerFunc(s.pools)
	router.Path(strings.ToLower("/Pool/{address}")).HandlerFunc(s.pool)
	router.Path(strings.ToLower("/Pool/{address}/Delegators/Count")).HandlerFunc(s.poolDelegatorsCount)
	router.Path(strings.ToLower("/Pool/{address}/Delegators")).HandlerFunc(s.poolDelegators)

	if s.dynamicEndpointLoader != nil {
		router.PathPrefix(strings.ToLower("/Data/")).HandlerFunc(s.data)
		s.refreshDynamicEndpoints()
		go s.loopDynamicEndpointsRefreshing()
	}
}

func (s *httpServer) loopDynamicEndpointsRefreshing() {
	for {
		time.Sleep(time.Minute)
		s.refreshDynamicEndpoints()
	}
}

func (s *httpServer) data(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("data", r.RequestURI)
	defer s.pm.Complete(id)
	arr := strings.Split(strings.ToLower(r.RequestURI), "/api/data/")
	var method string
	if len(arr) > 1 {
		method = arr[1]
	}
	dynamicEndpoint, ok := s.dynamicEndpointsByMethod[method]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		WriteErrorResponse(w, errors.New("unknown method"), s.logger)
		return
	}
	res, err := s.service.DynamicEndpointData(dynamicEndpoint.DataSource, dynamicEndpoint.Limit)
	WriteResponse(w, res, err, s.logger)
}

func (s *httpServer) dumpLink(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("dumpLink", r.RequestURI)
	defer s.pm.Complete(id)
	WriteResponse(w, s.getDumpLink(), nil, s.logger)
}

// @Tags Search
// @Id Search
// @Param value query string true "value to search"
// @Success 200 {object} api.Response{result=[]types.Entity}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Search [get]
func (s *httpServer) search(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("search", r.RequestURI)
	defer s.pm.Complete(id)

	value := mux.Vars(r)["value"]
	if addr := keyToAddrOrEmpty(value); len(addr) > 0 {
		value = addr
	}
	resp, err := s.service.Search(value)
	WriteResponse(w, resp, err, s.logger)
}

func keyToAddrOrEmpty(pkHex string) string {
	b, err := hex.DecodeString(pkHex)
	if err != nil {
		return ""
	}
	key, err := crypto.ToECDSA(b)
	if err != nil {
		return ""
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex()
}

// @Tags Coins
// @Id Coins
// @Success 200 {object} api.Response{result=types.AllCoins}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Coins [get]
func (s *httpServer) coins(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("coins", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Coins()
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) txtTotalSupply(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("coins", r.RequestURI)
	defer s.pm.Complete(id)

	coins, err := s.service.Coins()
	var resp string
	if err == nil {
		resp = coins.TotalBalance.Add(coins.TotalStake).String()
	}
	WriteTextPlainResponse(w, resp, err, s.logger)
}

// @Tags Coins
// @Id CirculatingSupply
// @Param format query string false "result value format" ENUMS(full,short)
// @Success 200 {object} api.Response{result=string}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /CirculatingSupply [get]
func (s *httpServer) circulatingSupply(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("circulatingSupply", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	format := strings.ToLower(vars["format"])
	if len(format) > 0 && format != "short" && format != "full" {
		WriteErrorResponse(w, errors.Errorf("Unknown value format=%s", format), s.logger)
		return
	}

	full := "full" == strings.ToLower(vars["format"])

	resp, err := s.service.CirculatingSupply(s.frozenBalanceAddrs)
	if err == nil && full {
		WriteResponse(w, blockchain.ConvertToInt(resp).String(), err, s.logger)
		return
	}
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) txtCirculatingSupply(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("circulatingSupply", r.RequestURI)
	defer s.pm.Complete(id)

	amount, err := s.service.CirculatingSupply(s.frozenBalanceAddrs)
	var resp string
	if err == nil {
		resp = amount.String()
	}
	WriteTextPlainResponse(w, resp, err, s.logger)
}

// @Tags Upgrades
// @Id Upgrades
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.BlockSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Upgrades [get]
func (s *httpServer) upgrades(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("upgrades", r.RequestURI)
	defer s.pm.Complete(id)

	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.Upgrades(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochsCount
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epochs/Count [get]
func (s *httpServer) epochsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.EpochsCount()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id Epochs
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.EpochSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epochs [get]
func (s *httpServer) epochs(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochs", r.RequestURI)
	defer s.pm.Complete(id)

	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.Epochs(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id LastEpoch
// @Success 200 {object} api.Response{result=types.EpochDetail}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/Last [get]
func (s *httpServer) lastEpoch(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("lastEpoch", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.LastEpoch()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id Epoch
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=types.EpochDetail}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch} [get]
func (s *httpServer) epoch(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epoch", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.Epoch(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochBlocksCount
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Blocks/Count [get]
func (s *httpServer) epochBlocksCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochBlocksCount", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochBlocksCount(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochBlocks
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.BlockSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Blocks [get]
func (s *httpServer) epochBlocks(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochBlocks", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochBlocks(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochFlipsCount
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Flips/Count [get]
func (s *httpServer) epochFlipsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochFlipsCount", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochFlipsCount(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochFlips
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.FlipSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Flips [get]
func (s *httpServer) epochFlips(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochFlips", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochFlips(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochFlipStatesSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.FlipStateCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/FlipStatesSummary [get]
func (s *httpServer) epochFlipStatesSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochFlipStatesSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochFlipStatesSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochFlipWrongWordsSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.NullableBoolValueCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/FlipWrongWordsSummary [get]
func (s *httpServer) epochFlipWrongWordsSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochFlipWrongWordsSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochFlipWrongWordsSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochIdentitiesCount
// @Param epoch path integer true "epoch"
// @Param states[] query []string false "identity state filter"
// @Param prevStates[] query []string false "identity previous state filter"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identities/Count [get]
func (s *httpServer) epochIdentitiesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentitiesCount", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentitiesCount(epoch, convertStates(r.Form["prevstates[]"]),
		convertStates(r.Form["states[]"]))
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochIdentities
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Param states[] query []string false "identity state filter"
// @Param prevStates[] query []string false "identity previous state filter"
// @Success 200 {object} api.ResponsePage{result=[]types.EpochIdentity}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identities [get]
func (s *httpServer) epochIdentities(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentities", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochIdentities(epoch, convertStates(r.Form["prevstates[]"]),
		convertStates(r.Form["states[]"]), count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func convertStates(formValues []string) []string {
	if len(formValues) == 0 {
		return nil
	}
	var res []string
	for _, formValue := range formValues {
		states := strings.Split(formValue, ",")
		for _, state := range states {
			if len(state) == 0 {
				continue
			}
			res = append(res, strings.ToUpper(state[0:1])+strings.ToLower(state[1:]))
		}
	}
	return res
}

func getFormValue(formValues url.Values, name string) string {
	if len(formValues[name]) == 0 {
		return ""
	}
	return formValues[name][0]
}

// @Tags Epochs
// @Id EpochIdentityStatesSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.IdentityStateCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/IdentityStatesSummary [get]
func (s *httpServer) epochIdentityStatesSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityStatesSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityStatesSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochIdentityStatesInterimSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.IdentityStateCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/IdentityStatesInterimSummary [get]
func (s *httpServer) epochIdentityStatesInterimSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityStatesInterimSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityStatesInterimSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochInvitesSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=types.InvitesSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/InvitesSummary [get]
func (s *httpServer) epochInvitesSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochInvitesSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochInvitesSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochInviteStatesSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.IdentityStateCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/InviteStatesSummary [get]
func (s *httpServer) epochInviteStatesSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochInviteStatesSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochInviteStatesSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochInvitesCount
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Invites/Count [get]
func (s *httpServer) epochInvitesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochInvitesCount", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochInvitesCount(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochInvites
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Invite}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Invites [get]
func (s *httpServer) epochInvites(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochInvites", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochInvites(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochTxsCount
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Txs/Count [get]
func (s *httpServer) epochTxsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochTxsCount", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochTxsCount(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochTxs
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.TransactionSummary{data=types.TransactionSpecificData}}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Txs [get]
func (s *httpServer) epochTxs(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochTxs", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochTxs(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochCoins
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=types.AllCoins}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Coins [get]
func (s *httpServer) epochCoins(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochCoins", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochCoins(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochRewardsSummary
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=types.RewardsSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/RewardsSummary [get]
func (s *httpServer) epochRewardsSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochRewardsSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochRewardsSummary(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochBadAuthorsCount
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Authors/Bad/Count [get]
func (s *httpServer) epochBadAuthorsCount(w http.ResponseWriter, r *http.Request) {
	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochBadAuthorsCount(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochBadAuthors
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.BadAuthor}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Authors/Bad [get]
func (s *httpServer) epochBadAuthors(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochBadAuthors", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochBadAuthors(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochIdentitiesRewardsCount
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/IdentityRewards/Count [get]
func (s *httpServer) epochIdentitiesRewardsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentitiesRewardsCount", r.RequestURI)
	defer s.pm.Complete(id)

	epoch, err := ReadUint(mux.Vars(r), "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentitiesRewardsCount(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochIdentitiesRewards
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Rewards}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/IdentityRewards [get]
func (s *httpServer) epochIdentitiesRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentitiesRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochIdentitiesRewards(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Epochs
// @Id EpochFundPayments
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.FundPayment}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/FundPayments [get]
func (s *httpServer) epochFundPayments(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochFundPayments", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochFundPayments(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochRewardBounds
// @Param epoch path integer true "epoch"
// @Success 200 {object} api.Response{result=[]types.RewardBounds}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/RewardBounds [get]
func (s *httpServer) epochRewardBounds(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochRewardBounds", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochRewardBounds(epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Epochs
// @Id EpochDelegateeTotalRewards
// @Param epoch path integer true "epoch"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.DelegateeTotalRewards}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/DelegateeTotalRewards [get]
func (s *httpServer) epochDelegateeTotalRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochDelegateeTotalRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochDelegateeTotalRewards(epoch, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentity
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.EpochIdentity}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address} [get]
func (s *httpServer) epochIdentity(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentity", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentity(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityShortFlipsToSolve
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]string}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/FlipsToSolve/Short [get]
func (s *httpServer) epochIdentityShortFlipsToSolve(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityShortFlipsToSolve", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityShortFlipsToSolve(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityLongFlipsToSolve
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]string}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/FlipsToSolve/Long [get]
func (s *httpServer) epochIdentityLongFlipsToSolve(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityLongFlipsToSolve", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityLongFlipsToSolve(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityShortAnswers
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.Answer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/Answers/Short [get]
func (s *httpServer) epochIdentityShortAnswers(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityShortAnswers", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityShortAnswers(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityLongAnswers
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.Answer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/Answers/Long [get]
func (s *httpServer) epochIdentityLongAnswers(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityLongAnswers", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityLongAnswers(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityFlips
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.FlipSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/Flips [get]
func (s *httpServer) epochIdentityFlips(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityFlips", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityFlips(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityRewardedFlips
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.FlipWithRewardFlag}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/RewardedFlips [get]
func (s *httpServer) epochIdentityFlipsWithRewardFlag(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityFlipsWithRewardFlag", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityFlipsWithRewardFlag(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityReportedFlipRewards
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.ReportedFlipReward}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/ReportRewards [get]
func (s *httpServer) epochIdentityReportedFlipRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityReportedFlipRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityReportedFlipRewards(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityBadAuthor
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.BadAuthor}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/Authors/Bad [get]
func (s *httpServer) epochIdentityBadAuthor(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityBadAuthor", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityBadAuthor(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityRewards
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.Reward}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/Rewards [get]
func (s *httpServer) epochIdentityRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityRewards(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityRewardedInvites
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.InviteWithRewardFlag}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/RewardedInvites [get]
func (s *httpServer) epochIdentityInvitesWithRewardFlag(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityInvitesWithRewardFlag", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityInvitesWithRewardFlag(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentitySavedInviteRewards
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.SavedInviteRewardCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/SavedInviteRewards [get]
func (s *httpServer) epochIdentitySavedInviteRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentitySavedInviteRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentitySavedInviteRewards(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityAvailableInvites
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.EpochInvites}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/AvailableInvites [get]
func (s *httpServer) epochIdentityAvailableInvites(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityAvailableInvites", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityAvailableInvites(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityValidationSummary
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.ValidationSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/ValidationSummary [get]
func (s *httpServer) epochIdentityValidationSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityValidationSummary", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochIdentityValidationSummary(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochIdentityDataWithProof
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=string}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Identity/{address}/DataWithProof [get]
func (s *httpServer) epochIdentityDataWithProof(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochIdentityDataWithProof", r.RequestURI)
	defer s.pm.Complete(id)
	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.IdentityWithProof(vars["address"], epoch)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id EpochAddressDelegateeRewards
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.Response{result=[]types.DelegateeReward}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Address/{address}/DelegateeRewards [get]
func (s *httpServer) epochDelegateeRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochDelegateeRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.EpochDelegateeRewards(epoch, vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Identity
// @Id EpochAddressDelegateeTotalRewards
// @Param epoch path integer true "epoch"
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.DelegateeTotalRewards}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Epoch/{epoch}/Address/{address}/DelegateeTotalRewards [get]
func (s *httpServer) epochAddressDelegateeTotalRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("epochAddressDelegateeTotalRewards", r.RequestURI)
	defer s.pm.Complete(id)

	vars := mux.Vars(r)
	epoch, err := ReadUint(vars, "epoch")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.EpochAddressDelegateeTotalRewards(epoch, vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Block
// @Id Block
// @Param id path string true "block hash or height"
// @Success 200 {object} api.Response{result=types.BlockDetail}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Block/{id} [get]
func (s *httpServer) block(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("block", r.RequestURI)
	defer s.pm.Complete(id)

	var resp interface{}
	vars := mux.Vars(r)
	height, err := ReadUint(vars, "id")
	if err != nil {
		resp, err = s.service.BlockByHash(vars["id"])
	} else {
		resp, err = s.service.BlockByHeight(height)
	}
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Block
// @Id BlockTxsCount
// @Param id path string true "block hash or height"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Block/{id}/Txs/Count [get]
func (s *httpServer) blockTxsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("blockTxsCount", r.RequestURI)
	defer s.pm.Complete(id)

	var resp interface{}
	vars := mux.Vars(r)
	height, err := ReadUint(vars, "id")
	if err != nil {
		resp, err = s.service.BlockTxsCountByHash(vars["id"])
	} else {
		resp, err = s.service.BlockTxsCountByHeight(height)
	}
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Block
// @Id BlockTxs
// @Param id path string true "block hash or height"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.TransactionSummary{data=types.TransactionSpecificData}}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Block/{id}/Txs [get]
func (s *httpServer) blockTxs(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("blockTxs", r.RequestURI)
	defer s.pm.Complete(id)

	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	height, err := ReadUint(vars, "id")
	var resp interface{}
	var nextContinuationToken *string
	if err != nil {
		resp, nextContinuationToken, err = s.service.BlockTxsByHash(vars["id"], count, continuationToken)
	} else {
		resp, nextContinuationToken, err = s.service.BlockTxsByHeight(height, count, continuationToken)
	}
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Block
// @Id BlockCoins
// @Param id path string true "block hash or height"
// @Success 200 {object} api.Response{result=types.AllCoins}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Block/{id}/Coins [get]
func (s *httpServer) blockCoins(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("blockCoins", r.RequestURI)
	defer s.pm.Complete(id)

	var resp interface{}
	vars := mux.Vars(r)
	height, err := ReadUint(vars, "id")
	if err != nil {
		resp, err = s.service.BlockCoinsByHash(vars["id"])
	} else {
		resp, err = s.service.BlockCoinsByHeight(height)
	}
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Block
// @Id LastBlock
// @Success 200 {object} api.Response{result=types.BlockDetail}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Block/Last [get]
func (s *httpServer) lastBlock(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("lastBlock", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.LastBlock()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id Identity
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.Identity}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address} [get]
func (s *httpServer) identity(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identity", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Identity(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityAge
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Age [get]
func (s *httpServer) identityAge(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityAge", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityAge(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityCurrentFlipCids
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]string}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/CurrentFlipCids [get]
func (s *httpServer) identityCurrentFlipCids(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityCurrentFlipCids", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityCurrentFlipCids(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityEpochsCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Epochs/Count [get]
func (s *httpServer) identityEpochsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityEpochsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityEpochsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityEpochs
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.EpochIdentity}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Epochs [get]
func (s *httpServer) identityEpochs(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityEpochs", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.IdentityEpochs(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Identity
// @Id IdentityFlipsCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Flips/Count [get]
func (s *httpServer) identityFlipsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityFlipsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityFlipsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityFlips
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.FlipSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Flips [get]
func (s *httpServer) identityFlips(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityFlips", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.IdentityFlips(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Identity
// @Id IdentityFlipStates
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.FlipStateCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/FlipStates [get]
func (s *httpServer) identityFlipStates(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityFlipStates", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityFlipStates(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityFlipQualifiedAnswers
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=[]types.FlipAnswerCount}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/FlipQualifiedAnswers [get]
func (s *httpServer) identityFlipRightAnswers(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityFlipRightAnswers", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityFlipQualifiedAnswers(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityInvitesCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Invites/Count [get]
func (s *httpServer) identityInvitesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityInvitesCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityInvitesCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityInvites
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Invite}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Invites [get]
func (s *httpServer) identityInvites(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityInvites", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.IdentityInvites(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Address
// @Id AddressTxsCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/Txs/Count [get]
func (s *httpServer) identityTxsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityTxsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityTxsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Address
// @Id AddressTxs
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.TransactionSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/Txs [get]
func (s *httpServer) identityTxs(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityTxs", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.IdentityTxs(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Identity
// @Id IdentityRewardsCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Rewards/Count [get]
func (s *httpServer) identityRewardsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityRewardsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityRewardsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityRewards
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Reward}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/Rewards [get]
func (s *httpServer) identityRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityRewards", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.IdentityRewards(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Identity
// @Id IdentityEpochRewardsCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/EpochRewards/Count [get]
func (s *httpServer) identityEpochRewardsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityEpochRewardsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.IdentityEpochRewardsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Identity
// @Id IdentityEpochRewards
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Rewards}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Identity/{address}/EpochRewards [get]
func (s *httpServer) identityEpochRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("identityEpochRewards", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.IdentityEpochRewards(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Flip
// @Id Flip
// @Param hash path string true "flip hash"
// @Success 200 {object} api.Response{result=types.Flip}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Flip/{hash} [get]
func (s *httpServer) flip(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("flip", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Flip(mux.Vars(r)["hash"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Flip
// @Id FlipContent
// @Param hash path string true "flip hash"
// @Success 200 {object} api.Response{result=types.FlipContent}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Flip/{hash}/Content [get]
func (s *httpServer) flipContent(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("flipContent", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.FlipContent(mux.Vars(r)["hash"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Flip
// @Id FlipShortAnswers
// @Param hash path string true "flip hash"
// @Success 200 {object} api.Response{result=[]types.Answer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Flip/{hash}/Answers/Short [get]
func (s *httpServer) flipShortAnswers(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("flipShortAnswers", r.RequestURI)
	defer s.pm.Complete(id)

	s.flipAnswers(w, r, true)
}

// @Tags Flip
// @Id FlipLongAnswers
// @Param hash path string true "flip hash"
// @Success 200 {object} api.Response{result=[]types.Answer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Flip/{hash}/Answers/Long [get]
func (s *httpServer) flipLongAnswers(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("flipLongAnswers", r.RequestURI)
	defer s.pm.Complete(id)

	s.flipAnswers(w, r, false)
}

func (s *httpServer) flipAnswers(w http.ResponseWriter, r *http.Request, isShort bool) {
	vars := mux.Vars(r)
	resp, err := s.service.FlipAnswers(vars["hash"], isShort)
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) flipEpochAdjacentFlips(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("flipEpochAdjacentFlips", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.FlipEpochAdjacentFlips(mux.Vars(r)["hash"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Address
// @Id Address
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.Address}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address} [get]
func (s *httpServer) address(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("address", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Address(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Address
// @Id AddressPenaltiesCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/Penalties/Count [get]
func (s *httpServer) addressPenaltiesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressPenaltiesCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.AddressPenaltiesCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Address
// @Id AddressPenalties
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Penalty}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/Penalties [get]
func (s *httpServer) addressPenalties(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressPenalties", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.AddressPenalties(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) addressStatesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressStatesCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.AddressStatesCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) addressStates(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressStates", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.AddressStates(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) addressTotalLatestMiningReward(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressTotalLatestMiningReward", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.AddressTotalLatestMiningReward(s.getOffsetUTC(), mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) addressTotalLatestBurntCoins(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressTotalLatestBurntCoins", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.AddressTotalLatestBurntCoins(s.getOffsetUTC(), mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) addressBadAuthorsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressBadAuthorsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.AddressBadAuthorsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) addressBadAuthors(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressBadAuthors", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.AddressBadAuthors(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) addressBalanceUpdatesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressBalanceUpdatesCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.AddressBalanceUpdatesCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) addressBalanceUpdates(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressBalanceUpdates", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.AddressBalanceUpdates(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Address
// @Id AddressBalanceUpdatesSummary
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.BalanceUpdatesSummary}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/Balance/Changes/Summary [get]
func (s *httpServer) addressBalanceUpdatesSummary(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressBalanceUpdatesSummary", r.RequestURI)
	defer s.pm.Complete(id)
	vars := mux.Vars(r)
	resp, err := s.service.AddressBalanceUpdatesSummary(vars["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Address
// @Tags Pool
// @Id AddressDelegateeTotalRewards
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.DelegateeTotalRewards}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/DelegateeTotalRewards [get]
func (s *httpServer) addressDelegateeTotalRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressDelegateeTotalRewards", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.AddressDelegateeTotalRewards(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Transaction
// @Id Transaction
// @Param hash path string true "transaction hash"
// @Success 200 {object} api.Response{result=types.TransactionDetail{data=types.TransactionSpecificData}}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Transaction/{hash} [get]
func (s *httpServer) transaction(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("transaction", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Transaction(mux.Vars(r)["hash"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Transaction
// @Id TransactionRaw
// @Param hash path string true "transaction hash"
// @Success 200 {object} api.Response{result=string}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Transaction/{hash}/Raw [get]
func (s *httpServer) transactionRaw(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("transactionRaw", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.TransactionRaw(mux.Vars(r)["hash"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) balancesCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("balancesCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.BalancesCount()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Coins
// @Id Balances
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Balance}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Balances [get]
func (s *httpServer) balances(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("balances", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.Balances(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Coins
// @Id Staking
// @Success 200 {object} api.Response{result=types.Staking}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Staking [get]
func (s *httpServer) staking(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("staking", r.RequestURI)
	defer s.pm.Complete(id)
	resp, err := s.service.Staking()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags MemPool
// @Id MemPoolTxs
// @Param limit query integer true "items to take"
// @Success 200 {object} api.ResponsePage{result=[]types.TransactionSummary{data=types.TransactionSpecificData}}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /MemPool/Txs [get]
func (s *httpServer) memPoolTxs(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("memPoolTxs", r.RequestURI)
	defer s.pm.Complete(id)

	count, _, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.MemPoolTxs(count)
	WriteResponse(w, resp, err, s.logger)
}

// @Tags MemPool
// @Id MemPoolTxsCount
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /MemPool/Txs/Count [get]
func (s *httpServer) memPoolTxsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("memPoolTxsCount", r.RequestURI)
	defer s.pm.Complete(id)
	resp, err := s.service.MemPoolTxsCount()
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) getOffsetUTC() time.Time {
	return getOffsetUTC(s.latestHours)
}

func getOffsetUTC(hours int) time.Time {
	return time.Now().UTC().Add(-time.Hour * time.Duration(hours))
}

// @Tags Contracts
// @Id OracleVotingContracts
// @Param states[] query []string false "filter by voting states"
// @Param oracle query string false "oracle address"
// @Param all query boolean false "flag to return all voting contracts independently on oracle address"
// @Param sortBy query string false "value to sort" ENUMS(reward,timestamp)
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.OracleVotingContract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /OracleVotingContracts [get]
func (s *httpServer) oracleVotingContracts(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("oracleVotingContracts", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	all := getFormValue(r.Form, "all") == "true"
	address := mux.Vars(r)["address"]

	convertStates := func(formValues []string) []string {
		if len(formValues) == 0 {
			return nil
		}
		var res []string
		for _, formValue := range formValues {
			res = append(res, strings.Split(formValue, ",")...)
		}
		return res
	}
	states := convertStates(r.Form["states[]"])
	var sortBy *string
	if v := r.Form.Get("sortby"); len(v) > 0 {
		sortBy = &v
	}
	resp, nextContinuationToken, err := s.contractsService.OracleVotingContracts(address, getFormValue(r.Form, "oracle"),
		states, all, sortBy, count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Address
// @Tags Contracts
// @Id AddressOracleVotingContracts
// @Param address path string true "contract author address"
// @Param states[] query []string false "filter by voting states"
// @Param oracle query string false "oracle address"
// @Param all query boolean false "flag to return all voting contracts independently on oracle address"
// @Param sortBy query string false "value to sort" ENUMS(reward,timestamp)
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.OracleVotingContract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/OracleVotingContracts [get]
func (s *httpServer) addressOracleVotingContracts(w http.ResponseWriter, r *http.Request) {
	s.oracleVotingContracts(w, r)
}

// @Tags Contracts
// @Id TimeLockContract
// @Param address path string true "contract address"
// @Success 200 {object} api.Response{result=types.TimeLockContract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /TimeLockContract/{address} [get]
func (s *httpServer) timeLockContract(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("timeLockContract", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.TimeLockContract(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Contracts
// @Id OracleLockContract
// @Param address path string true "contract address"
// @Success 200 {object} api.Response{result=types.OracleLockContract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /OracleLockContract/{address} [get]
func (s *httpServer) oracleLockContract(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("oracleLockContract", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.OracleLockContract(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Contracts
// @Id OracleVotingContract
// @Param address path string true "contract address"
// @Param oracle query string false "oracle address"
// @Success 200 {object} api.Response{result=types.OracleVotingContract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /OracleVotingContract/{address} [get]
func (s *httpServer) oracleVotingContract(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("oracleVotingContract", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.contractsService.OracleVotingContract(mux.Vars(r)["address"], r.Form.Get("oracle"))
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Contracts
// @Id MultisigContract
// @Param address path string true "contract address"
// @Success 200 {object} api.Response{result=types.MultisigContract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /MultisigContract/{address} [get]
func (s *httpServer) multisigContract(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("multisigContract", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.MultisigContract(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Contracts
// @Id EstimatedOracleRewards
// @Success 200 {object} api.Response{result=[]types.EstimatedOracleReward}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /OracleVotingContracts/EstimatedOracleRewards [get]
func (s *httpServer) estimatedOracleRewards(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("estimatedOracleRewards", r.RequestURI)
	defer s.pm.Complete(id)
	resp, err := s.service.EstimatedOracleRewards()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Address
// @Tags Contracts
// @Id AddressContractTxBalanceUpdates
// @Param address path string true "address"
// @Param contractAddress path string true "contract address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.ContractTxBalanceUpdate}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Address/{address}/Contract/{contractAddress}/BalanceUpdates [get]
func (s *httpServer) addressContractTxBalanceUpdates(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("addressContractTxBalanceUpdates", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.contractsService.AddressContractTxBalanceUpdates(vars["address"], vars["contractaddress"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Contracts
// @Id Contract
// @Param address path string true "contract address"
// @Success 200 {object} api.ResponsePage{result=types.Contract}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Contract/{address} [get]
func (s *httpServer) contract(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("contract", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Contract(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Contracts
// @Id ContractTxBalanceUpdates
// @Param address path string true "contract address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.ContractTxBalanceUpdate}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Contract/{address}/BalanceUpdates [get]
func (s *httpServer) contractTxBalanceUpdates(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("contractTxBalanceUpdates", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.contractsService.ContractTxBalanceUpdates(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) onlineIdentitiesCount(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.GetOnlineIdentitiesCount()
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) onlineIdentities(w http.ResponseWriter, r *http.Request) {
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.GetOnlineIdentities(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) onlineIdentity(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.GetOnlineIdentity(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) onlineCount(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.GetOnlineCount()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Miners
// @Id MinersHistory
// @Success 200 {object} api.ResponsePage{result=[]types.MinersHistoryItem}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Miners/History [get]
func (s *httpServer) minersHistory(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("minersHistory", r.RequestURI)
	defer s.pm.Complete(id)
	resp, err := s.service.MinersHistory()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Peers
// @Id PeersHistory
// @Param limit query integer false "items to take"
// @Success 200 {object} api.ResponsePage{result=[]types.PeersHistoryItem}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Peers/History [get]
func (s *httpServer) peersHistory(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("peersHistory", r.RequestURI)
	defer s.pm.Complete(id)
	count, err := readPeersHistoryCount(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.PeersHistory(count)
	WriteResponse(w, resp, err, s.logger)
}

func readPeersHistoryCount(params url.Values) (uint64, error) {
	const defaultValue = uint64(500)
	if len(params.Get("limit")) == 0 {
		return defaultValue, nil
	}
	count, err := ReadUintUrlValue(params, "limit")
	if err != nil {
		return 0, err
	}
	if count > defaultValue {
		count = defaultValue
	}
	return count, nil
}

func (s *httpServer) validatorsCount(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.ValidatorsCount()
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) validators(w http.ResponseWriter, r *http.Request) {
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.Validators(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) onlineValidatorsCount(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.OnlineValidatorsCount()
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) onlineValidators(w http.ResponseWriter, r *http.Request) {
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.OnlineValidators(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) signatureAddress(w http.ResponseWriter, r *http.Request) {
	value := mux.Vars(r)["value"]
	signature := mux.Vars(r)["signature"]
	resp, err := s.service.SignatureAddress(value, signature)
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) upgradeVotings(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("upgradeVotings", r.RequestURI)
	defer s.pm.Complete(id)

	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.UpgradeVotings(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

func (s *httpServer) upgradeVoting(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.UpgradeVoting()
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) upgradeVotingHistory(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("upgrade", r.RequestURI)
	defer s.pm.Complete(id)

	upgrade, err := ReadUint(mux.Vars(r), "upgrade")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.UpgradeVotingHistory(upgrade)
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) upgrade(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("upgrade", r.RequestURI)
	defer s.pm.Complete(id)

	upgrade, err := ReadUint(mux.Vars(r), "upgrade")
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, err := s.service.Upgrade(upgrade)
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) forkChangeLog(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("forkChangeLog", r.RequestURI)
	defer s.pm.Complete(id)

	version := mux.Vars(r)["version"]
	resp, err := s.service.ForkChangeLog(version)
	WriteResponse(w, resp, err, s.logger)
}

func (s *httpServer) now(w http.ResponseWriter, r *http.Request) {
	WriteResponse(w, time.Now().UTC().Truncate(time.Second), nil, s.logger)
}

// @Tags Pools
// @Id PoolsCount
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Pools/Count [get]
func (s *httpServer) poolsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("poolsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.PoolsCount()
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Pools
// @Id Pools
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Pool}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Pools [get]
func (s *httpServer) pools(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("pools", r.RequestURI)
	defer s.pm.Complete(id)

	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	resp, nextContinuationToken, err := s.service.Pools(count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}

// @Tags Pools
// @Id Pool
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=types.Pool}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Pool/{address} [get]
func (s *httpServer) pool(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("pool", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.Pool(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Pools
// @Id PoolDelegatorsCount
// @Param address path string true "address"
// @Success 200 {object} api.Response{result=integer}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Pool/{address}/Delegators/Count [get]
func (s *httpServer) poolDelegatorsCount(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("poolDelegatorsCount", r.RequestURI)
	defer s.pm.Complete(id)

	resp, err := s.service.PoolDelegatorsCount(mux.Vars(r)["address"])
	WriteResponse(w, resp, err, s.logger)
}

// @Tags Pools
// @Id PoolDelegators
// @Param address path string true "address"
// @Param limit query integer true "items to take"
// @Param continuationToken query string false "continuation token to get next page items"
// @Success 200 {object} api.ResponsePage{result=[]types.Delegator}
// @Failure 400 "Bad request"
// @Failure 429 "Request number limit exceeded"
// @Failure 500 "Internal server error"
// @Failure 503 "Service unavailable"
// @Router /Pool/{address}/Delegators [get]
func (s *httpServer) poolDelegators(w http.ResponseWriter, r *http.Request) {
	id := s.pm.Start("poolDelegators", r.RequestURI)
	defer s.pm.Complete(id)
	count, continuationToken, err := ReadPaginatorParams(r.Form)
	if err != nil {
		WriteErrorResponse(w, err, s.logger)
		return
	}
	vars := mux.Vars(r)
	resp, nextContinuationToken, err := s.service.PoolDelegators(vars["address"], count, continuationToken)
	WriteResponsePage(w, resp, nextContinuationToken, err, s.logger)
}
