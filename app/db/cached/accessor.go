package cached

import (
	"fmt"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-indexer-api/app/api"
	"github.com/idena-network/idena-indexer-api/app/db"
	"github.com/idena-network/idena-indexer-api/app/db/postgres"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	permanentDataLifeTime                   = time.Hour * 2
	activeAddressesCountMethod              = "ActiveAddressesCount"
	epochFlipAnswersSummaryMethod           = "EpochFlipAnswersSummary"
	epochFlipStatesSummaryMethod            = "EpochFlipStatesSummary"
	epochFlipWrongWordsSummaryMethod        = "EpochFlipWrongWordsSummary"
	epochIdentityStatesSummaryMethod        = "EpochIdentityStatesSummary"
	epochIdentityStatesInterimSummaryMethod = "EpochIdentityStatesInterimSummary"
	epochInvitesSummaryMethod               = "EpochInvitesSummary"
	epochInviteStatesSummaryMethod          = "EpochInviteStatesSummary"
	epochRewardsSummaryMethod               = "EpochRewardsSummary"
	epochBadAuthorsCountMethod              = "EpochBadAuthorsCount"
	epochBadAuthorsMethod                   = "EpochBadAuthors"
	epochBadAuthorsOldMethod                = "EpochBadAuthorsOld"
	epochIdentitiesRewardsCountMethod       = "EpochIdentitiesRewardsCount"
	epochIdentitiesRewardsMethod            = "EpochIdentitiesRewards"
	epochIdentitiesRewardsOldMethod         = "EpochIdentitiesRewardsOld"
	epochFundPaymentsMethod                 = "EpochFundPayments"
	epochRewardBoundsMethod                 = "EpochRewardBounds"
	flipEpochAdjacentFlipsMethod            = "FlipEpochAdjacentFlips"
	flipAddressAdjacentFlipsMethod          = "FlipAddressAdjacentFlips"
	flipEpochIdentityAdjacentFlipsMethod    = "FlipEpochIdentityAdjacentFlips"
	upgradeMethod                           = "Upgrade"
	epochIdentityMethod                     = "EpochIdentity"
	lastBlock                               = "LastBlock"
)

type cachedAccessor struct {
	accessor                 db.Accessor
	memPool                  api.MemPool
	eventBus                 eventbus.Bus
	maxItemCountsByMethod    map[string]int
	defaultCacheMaxItemCount int
	maxItemLifeTimesByMethod map[string]time.Duration
	defaultCacheItemLifeTime time.Duration
	cachesByMethod           map[string]Cache
	mutex                    sync.Mutex
	logger                   log.Logger
}

func NewCachedAccessor(
	db db.Accessor,
	memPool api.MemPool,
	defaultCacheMaxItemCount int,
	defaultCacheItemLifeTime time.Duration,
	logger log.Logger,
) db.Accessor {
	a := &cachedAccessor{
		accessor:                 db,
		maxItemCountsByMethod:    createMaxItemCountsByMethod(),
		defaultCacheMaxItemCount: defaultCacheMaxItemCount,
		maxItemLifeTimesByMethod: createMaxItemLifeTimesByMethod(),
		defaultCacheItemLifeTime: defaultCacheItemLifeTime,
		logger:                   logger,
		memPool:                  memPool,
	}
	go func() {
		for {
			time.Sleep(time.Minute)
			a.log()
		}
	}()
	go a.monitorEpochChange()
	return a
}

func createMaxItemCountsByMethod() map[string]int {
	return map[string]int{
		activeAddressesCountMethod: 1,
	}
}

func createMaxItemLifeTimesByMethod() map[string]time.Duration {
	return map[string]time.Duration{
		lastBlock:                               time.Second * 20,
		activeAddressesCountMethod:              time.Minute * 5,
		epochIdentityStatesInterimSummaryMethod: time.Minute * 5,
		epochInvitesSummaryMethod:               time.Minute * 3,
		epochInviteStatesSummaryMethod:          time.Minute * 3,
		flipEpochAdjacentFlipsMethod:            time.Minute * 20,
		flipAddressAdjacentFlipsMethod:          time.Minute * 20,
		flipEpochIdentityAdjacentFlipsMethod:    time.Minute * 20,
		epochFlipAnswersSummaryMethod:           permanentDataLifeTime,
		epochFlipStatesSummaryMethod:            permanentDataLifeTime,
		epochFlipWrongWordsSummaryMethod:        permanentDataLifeTime,
		epochIdentityStatesSummaryMethod:        permanentDataLifeTime,
		epochRewardsSummaryMethod:               permanentDataLifeTime,
		epochBadAuthorsCountMethod:              permanentDataLifeTime,
		epochBadAuthorsMethod:                   permanentDataLifeTime,
		epochBadAuthorsOldMethod:                permanentDataLifeTime,
		epochIdentitiesRewardsCountMethod:       permanentDataLifeTime,
		epochIdentitiesRewardsMethod:            permanentDataLifeTime,
		epochIdentitiesRewardsOldMethod:         permanentDataLifeTime,
		epochFundPaymentsMethod:                 permanentDataLifeTime,
		epochRewardBoundsMethod:                 permanentDataLifeTime,
		upgradeMethod:                           permanentDataLifeTime,
		epochIdentityMethod:                     permanentDataLifeTime,
	}
}

func (a *cachedAccessor) monitorEpochChange() {
	isFirst := true
	epoch := uint64(0)
	const delay = time.Second * 5
	for {
		time.Sleep(delay)
		lastEpoch, err := a.accessor.LastEpoch()
		if err != nil {
			a.logger.Warn(errors.Wrap(err, "Unable to get last epoch from db to detect new one").Error())
			continue
		}
		a.logger.Debug(fmt.Sprintf("epoch: %v, lastEpoch: %v", epoch, lastEpoch.Epoch))
		if lastEpoch.Epoch > epoch {
			epoch = lastEpoch.Epoch
			if isFirst {
				isFirst = false
			} else {
				a.logger.Debug("Detected new epoch")
				a.clearCache()
			}
		}
		timeToStartMonitoring := lastEpoch.ValidationTime.Add(time.Minute * 25)
		now := time.Now()
		if timeToStartMonitoring.After(now) {
			<-time.After(timeToStartMonitoring.Sub(now))
		}
	}
}

func (a *cachedAccessor) clearCache() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	for method, dbCache := range a.cachesByMethod {
		dbCache.Clear()
		a.logger.Debug(fmt.Sprintf("Cleared %v cache", method))
	}
}

func (a *cachedAccessor) log() {
	type methodItemsCount struct {
		method string
		count  int
	}
	a.mutex.Lock()
	itemsCounts := make([]*methodItemsCount, len(a.cachesByMethod))
	i := 0
	for method, dbCache := range a.cachesByMethod {
		itemsCounts[i] = &methodItemsCount{
			method: method,
			count:  dbCache.ItemsCount(),
		}
		i++
	}
	a.mutex.Unlock()

	emptyCount := 0
	var s []string
	if len(itemsCounts) > 0 {
		sort.Slice(itemsCounts, func(i, j int) bool {
			return itemsCounts[i].count >= itemsCounts[j].count
		})
		for _, itemsCount := range itemsCounts {
			if itemsCount.count > 0 {
				s = append(s, fmt.Sprintf("%s: %d", itemsCount.method, itemsCount.count))
			} else {
				emptyCount++
			}
		}
	}
	header := fmt.Sprintf("Total: %d, empty: %d", len(itemsCounts), emptyCount)
	var infoToLog string
	if len(s) > 0 {
		infoToLog = fmt.Sprintf("%s (%s)", header, strings.Join(s, ", "))
	} else {
		infoToLog = header
	}
	a.logger.Debug(infoToLog)
}

type cachedValue struct {
	res               interface{}
	continuationToken *string
	err               error
}

func key(args ...interface{}) string {
	res := "key"
	for _, arg := range args {
		res = fmt.Sprintf("%s-%v", res, arg)
	}
	return res
}

func (a *cachedAccessor) getOrLoad(method string, load func() (interface{}, error), args ...interface{}) (interface{}, error) {
	dbCache := a.getCache(method)
	key := key(args)
	if v, ok := dbCache.Get(key); ok {
		return v.(*cachedValue).res, v.(*cachedValue).err
	}
	res, err := load()
	dbCache.Set(key, &cachedValue{
		res: res,
		err: err,
	}, cache.DefaultExpiration)
	return res, err
}

func (a *cachedAccessor) getOrLoadWithConToken(method string, load func() (interface{}, *string, error), args ...interface{}) (interface{}, *string, error) {
	dbCache := a.getCache(method)
	key := key(args)
	if v, ok := dbCache.Get(key); ok {
		return v.(*cachedValue).res, v.(*cachedValue).continuationToken, v.(*cachedValue).err
	}
	res, continuationToken, err := load()
	dbCache.Set(key, &cachedValue{
		res:               res,
		continuationToken: continuationToken,
		err:               err,
	}, cache.DefaultExpiration)
	return res, continuationToken, err
}

func (a *cachedAccessor) getCache(method string) Cache {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.cachesByMethod == nil {
		a.cachesByMethod = make(map[string]Cache)
	}
	dbCache, ok := a.cachesByMethod[method]
	if !ok {
		maxSize := a.defaultCacheMaxItemCount
		if count, ok := a.maxItemCountsByMethod[method]; ok {
			maxSize = count
		}
		defaultExpiration := a.defaultCacheItemLifeTime
		if lifeTime, ok := a.maxItemLifeTimesByMethod[method]; ok {
			defaultExpiration = lifeTime
		}
		dbCache = NewCache(
			maxSize,
			defaultExpiration,
			a.logger.New("component", fmt.Sprintf("cache-%s", method)),
		)
		a.cachesByMethod[method] = dbCache
	}
	return dbCache
}

func (a *cachedAccessor) Search(value string) ([]types.Entity, error) {
	res, err := a.getOrLoad("Search", func() (interface{}, error) {
		return a.accessor.Search(value)
	}, value)
	return res.([]types.Entity), err
}

func (a *cachedAccessor) Coins() (types.AllCoins, error) {
	res, err := a.getOrLoad("Coins", func() (interface{}, error) {
		return a.accessor.Coins()
	})
	return res.(types.AllCoins), err
}

func (a *cachedAccessor) CirculatingSupply(addressesToExclude []string) (decimal.Decimal, error) {
	res, err := a.getOrLoad("CirculatingSupply", func() (interface{}, error) {
		return a.accessor.CirculatingSupply(addressesToExclude)
	})
	return res.(decimal.Decimal), err
}

func (a *cachedAccessor) ActiveAddressesCount(afterTime time.Time) (uint64, error) {
	res, err := a.getOrLoad(activeAddressesCountMethod, func() (interface{}, error) {
		return a.accessor.ActiveAddressesCount(afterTime)
	})
	return res.(uint64), err
}

func (a *cachedAccessor) EpochsCount() (uint64, error) {
	res, err := a.getOrLoad("EpochsCount", func() (interface{}, error) {
		return a.accessor.EpochsCount()
	})
	return res.(uint64), err
}

func (a *cachedAccessor) Epochs(count uint64, continuationToken *string) ([]types.EpochSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("Epochs", func() (interface{}, *string, error) {
		return a.accessor.Epochs(count, continuationToken)
	}, count, continuationToken)
	return res.([]types.EpochSummary), nextContinuationToken, err
}

func (a *cachedAccessor) LastEpoch() (types.EpochDetail, error) {
	res, err := a.getOrLoad("LastEpoch", func() (interface{}, error) {
		return a.accessor.LastEpoch()
	})
	return res.(types.EpochDetail), err
}

func (a *cachedAccessor) Epoch(epoch uint64) (types.EpochDetail, error) {
	res, err := a.getOrLoad("Epoch", func() (interface{}, error) {
		return a.accessor.Epoch(epoch)
	}, epoch)
	return res.(types.EpochDetail), err
}

func (a *cachedAccessor) EpochBlocksCount(epoch uint64) (uint64, error) {
	res, err := a.getOrLoad("EpochBlocksCount", func() (interface{}, error) {
		return a.accessor.EpochBlocksCount(epoch)
	}, epoch)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochBlocks(epoch uint64, count uint64, continuationToken *string) ([]types.BlockSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochBlocks", func() (interface{}, *string, error) {
		return a.accessor.EpochBlocks(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.BlockSummary), nextContinuationToken, err
}

func (a *cachedAccessor) EpochFlipsCount(epoch uint64) (uint64, error) {
	res, err := a.getOrLoad("EpochFlipsCount", func() (interface{}, error) {
		return a.accessor.EpochFlipsCount(epoch)
	}, epoch)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochFlips(epoch uint64, count uint64, continuationToken *string) ([]types.FlipSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochFlips", func() (interface{}, *string, error) {
		return a.accessor.EpochFlips(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.FlipSummary), nextContinuationToken, err
}

func (a *cachedAccessor) EpochFlipAnswersSummary(epoch uint64) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad(epochFlipAnswersSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochFlipAnswersSummary(epoch)
	}, epoch)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) EpochFlipStatesSummary(epoch uint64) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad(epochFlipStatesSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochFlipStatesSummary(epoch)
	}, epoch)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) EpochFlipWrongWordsSummary(epoch uint64) ([]types.NullableBoolValueCount, error) {
	res, err := a.getOrLoad(epochFlipWrongWordsSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochFlipWrongWordsSummary(epoch)
	}, epoch)
	return res.([]types.NullableBoolValueCount), err
}

func (a *cachedAccessor) EpochIdentitiesCount(epoch uint64, prevStates []string, states []string) (uint64, error) {
	res, err := a.getOrLoad("EpochIdentitiesCount", func() (interface{}, error) {
		return a.accessor.EpochIdentitiesCount(epoch, prevStates, states)
	}, epoch, prevStates, states)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochIdentities(epoch uint64, prevStates []string, states []string, count uint64,
	continuationToken *string) ([]types.EpochIdentity, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochIdentities", func() (interface{}, *string, error) {
		return a.accessor.EpochIdentities(epoch, prevStates, states, count, continuationToken)
	}, epoch, prevStates, states, count, continuationToken)
	return res.([]types.EpochIdentity), nextContinuationToken, err
}

func (a *cachedAccessor) EpochIdentityStatesSummary(epoch uint64) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad(epochIdentityStatesSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochIdentityStatesSummary(epoch)
	}, epoch)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) EpochIdentityStatesInterimSummary(epoch uint64) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad(epochIdentityStatesInterimSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochIdentityStatesInterimSummary(epoch)
	}, epoch)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) EpochInvitesSummary(epoch uint64) (types.InvitesSummary, error) {
	res, err := a.getOrLoad(epochInvitesSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochInvitesSummary(epoch)
	}, epoch)
	return res.(types.InvitesSummary), err
}

func (a *cachedAccessor) EpochInviteStatesSummary(epoch uint64) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad(epochInviteStatesSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochInviteStatesSummary(epoch)
	}, epoch)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) EpochInvitesCount(epoch uint64) (uint64, error) {
	res, err := a.getOrLoad("EpochInvitesCount", func() (interface{}, error) {
		return a.accessor.EpochInvitesCount(epoch)
	}, epoch)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochInvites(epoch uint64, count uint64, continuationToken *string) ([]types.Invite, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochInvites", func() (interface{}, *string, error) {
		return a.accessor.EpochInvites(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.Invite), nextContinuationToken, err
}

func (a *cachedAccessor) EpochTxsCount(epoch uint64) (uint64, error) {
	res, err := a.getOrLoad("EpochTxsCount", func() (interface{}, error) {
		return a.accessor.EpochTxsCount(epoch)
	}, epoch)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochTxs(epoch uint64, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochTxs", func() (interface{}, *string, error) {
		return a.accessor.EpochTxs(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.TransactionSummary), nextContinuationToken, err
}

func (a *cachedAccessor) EpochCoins(epoch uint64) (types.AllCoins, error) {
	res, err := a.getOrLoad("EpochCoins", func() (interface{}, error) {
		return a.accessor.EpochCoins(epoch)
	}, epoch)
	return res.(types.AllCoins), err
}

func (a *cachedAccessor) EpochRewardsSummary(epoch uint64) (types.RewardsSummary, error) {
	res, err := a.getOrLoad(epochRewardsSummaryMethod, func() (interface{}, error) {
		return a.accessor.EpochRewardsSummary(epoch)
	}, epoch)
	return res.(types.RewardsSummary), err
}

func (a *cachedAccessor) EpochBadAuthorsCount(epoch uint64) (uint64, error) {
	res, err := a.getOrLoad(epochBadAuthorsCountMethod, func() (interface{}, error) {
		return a.accessor.EpochBadAuthorsCount(epoch)
	}, epoch)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochBadAuthors(epoch uint64, count uint64, continuationToken *string) ([]types.BadAuthor, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken(epochBadAuthorsMethod, func() (interface{}, *string, error) {
		return a.accessor.EpochBadAuthors(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.BadAuthor), nextContinuationToken, err
}

func (a *cachedAccessor) EpochIdentitiesRewardsCount(epoch uint64) (uint64, error) {
	res, err := a.getOrLoad(epochIdentitiesRewardsCountMethod, func() (interface{}, error) {
		return a.accessor.EpochIdentitiesRewardsCount(epoch)
	}, epoch)
	return res.(uint64), err
}

func (a *cachedAccessor) EpochIdentitiesRewards(epoch uint64, count uint64, continuationToken *string) ([]types.Rewards, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken(epochIdentitiesRewardsMethod, func() (interface{}, *string, error) {
		return a.accessor.EpochIdentitiesRewards(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.Rewards), nextContinuationToken, err
}

func (a *cachedAccessor) EpochFundPayments(epoch uint64) ([]types.FundPayment, error) {
	res, err := a.getOrLoad(epochFundPaymentsMethod, func() (interface{}, error) {
		return a.accessor.EpochFundPayments(epoch)
	}, epoch)
	return res.([]types.FundPayment), err
}

func (a *cachedAccessor) EpochRewardBounds(epoch uint64) ([]types.RewardBounds, error) {
	res, err := a.getOrLoad(epochRewardBoundsMethod, func() (interface{}, error) {
		return a.accessor.EpochRewardBounds(epoch)
	}, epoch)
	return res.([]types.RewardBounds), err
}

func (a *cachedAccessor) EpochDelegateeTotalRewards(epoch uint64, count uint64, continuationToken *string) ([]types.DelegateeTotalRewards, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochDelegateeTotalRewards", func() (interface{}, *string, error) {
		return a.accessor.EpochDelegateeTotalRewards(epoch, count, continuationToken)
	}, epoch, count, continuationToken)
	return res.([]types.DelegateeTotalRewards), nextContinuationToken, err
}

func (a *cachedAccessor) EpochIdentity(epoch uint64, address string) (types.EpochIdentity, error) {
	res, err := a.getOrLoad(epochIdentityMethod, func() (interface{}, error) {
		return a.accessor.EpochIdentity(epoch, address)
	}, epoch, address)
	return res.(types.EpochIdentity), err
}

func (a *cachedAccessor) EpochIdentityShortFlipsToSolve(epoch uint64, address string) ([]string, error) {
	res, err := a.getOrLoad("EpochIdentityShortFlipsToSolve", func() (interface{}, error) {
		return a.accessor.EpochIdentityShortFlipsToSolve(epoch, address)
	}, epoch, address)
	return res.([]string), err
}

func (a *cachedAccessor) EpochIdentityLongFlipsToSolve(epoch uint64, address string) ([]string, error) {
	res, err := a.getOrLoad("EpochIdentityLongFlipsToSolve", func() (interface{}, error) {
		return a.accessor.EpochIdentityLongFlipsToSolve(epoch, address)
	}, epoch, address)
	return res.([]string), err
}

func (a *cachedAccessor) EpochIdentityShortAnswers(epoch uint64, address string) ([]types.Answer, error) {
	res, err := a.getOrLoad("EpochIdentityShortAnswers", func() (interface{}, error) {
		return a.accessor.EpochIdentityShortAnswers(epoch, address)
	}, epoch, address)
	return res.([]types.Answer), err
}

func (a *cachedAccessor) EpochIdentityLongAnswers(epoch uint64, address string) ([]types.Answer, error) {
	res, err := a.getOrLoad("EpochIdentityLongAnswers", func() (interface{}, error) {
		return a.accessor.EpochIdentityLongAnswers(epoch, address)
	}, epoch, address)
	return res.([]types.Answer), err
}

func (a *cachedAccessor) EpochIdentityFlips(epoch uint64, address string) ([]types.FlipSummary, error) {
	res, err := a.getOrLoad("EpochIdentityFlips", func() (interface{}, error) {
		return a.accessor.EpochIdentityFlips(epoch, address)
	}, epoch, address)
	return res.([]types.FlipSummary), err
}

func (a *cachedAccessor) EpochIdentityFlipsWithRewardFlag(epoch uint64, address string) ([]types.FlipWithRewardFlag, error) {
	res, err := a.getOrLoad("EpochIdentityFlipsWithRewardFlag", func() (interface{}, error) {
		return a.accessor.EpochIdentityFlipsWithRewardFlag(epoch, address)
	}, epoch, address)
	return res.([]types.FlipWithRewardFlag), err
}

func (a *cachedAccessor) EpochIdentityReportedFlipRewards(epoch uint64, address string) ([]types.ReportedFlipReward, error) {
	res, err := a.getOrLoad("EpochIdentityReportedFlipRewards", func() (interface{}, error) {
		return a.accessor.EpochIdentityReportedFlipRewards(epoch, address)
	}, epoch, address)
	return res.([]types.ReportedFlipReward), err
}

func (a *cachedAccessor) EpochIdentityRewards(epoch uint64, address string) ([]types.Reward, error) {
	res, err := a.getOrLoad("EpochIdentityRewards", func() (interface{}, error) {
		return a.accessor.EpochIdentityRewards(epoch, address)
	}, epoch, address)
	return res.([]types.Reward), err
}

func (a *cachedAccessor) EpochIdentityBadAuthor(epoch uint64, address string) (*types.BadAuthor, error) {
	res, err := a.getOrLoad("EpochIdentityBadAuthor", func() (interface{}, error) {
		return a.accessor.EpochIdentityBadAuthor(epoch, address)
	}, epoch, address)
	return res.(*types.BadAuthor), err
}

func (a *cachedAccessor) EpochIdentityInvitesWithRewardFlag(epoch uint64, address string) ([]types.InviteWithRewardFlag, error) {
	res, err := a.getOrLoad("EpochIdentityInvitesWithRewardFlag", func() (interface{}, error) {
		return a.accessor.EpochIdentityInvitesWithRewardFlag(epoch, address)
	}, epoch, address)
	return res.([]types.InviteWithRewardFlag), err
}

func (a *cachedAccessor) EpochIdentitySavedInviteRewards(epoch uint64, address string) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad("EpochIdentitySavedInviteRewards", func() (interface{}, error) {
		return a.accessor.EpochIdentitySavedInviteRewards(epoch, address)
	}, epoch, address)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) EpochIdentityAvailableInvites(epoch uint64, address string) ([]types.EpochInvites, error) {
	res, err := a.getOrLoad("EpochIdentityAvailableInvites", func() (interface{}, error) {
		return a.accessor.EpochIdentityAvailableInvites(epoch, address)
	}, epoch, address)
	return res.([]types.EpochInvites), err
}

func (a *cachedAccessor) EpochIdentityInviteeWithRewardFlag(epoch uint64, address string) (*types.InviteeWithRewardFlag, error) {
	res, err := a.getOrLoad("EpochIdentityInviteeWithRewardFlag", func() (interface{}, error) {
		return a.accessor.EpochIdentityInviteeWithRewardFlag(epoch, address)
	}, epoch, address)
	return res.(*types.InviteeWithRewardFlag), err
}

func (a *cachedAccessor) EpochIdentityValidationSummary(epoch uint64, address string) (types.ValidationSummary, error) {
	res, err := a.getOrLoad("EpochIdentityValidationSummary", func() (interface{}, error) {
		return a.accessor.EpochIdentityValidationSummary(epoch, address)
	}, epoch, address)
	return res.(types.ValidationSummary), err
}

func (a *cachedAccessor) EpochAddressDelegateeTotalRewards(epoch uint64, address string) (types.DelegateeTotalRewards, error) {
	res, err := a.getOrLoad("EpochAddressDelegateeTotalRewards", func() (interface{}, error) {
		return a.accessor.EpochAddressDelegateeTotalRewards(epoch, address)
	}, epoch, address)
	return res.(types.DelegateeTotalRewards), err
}

func (a *cachedAccessor) EpochDelegateeRewards(epoch uint64, address string, count uint64, continuationToken *string) ([]types.DelegateeReward, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("EpochDelegateeRewards", func() (interface{}, *string, error) {
		return a.accessor.EpochDelegateeRewards(epoch, address, count, continuationToken)
	}, epoch, address, count, continuationToken)
	return res.([]types.DelegateeReward), nextContinuationToken, err
}

func (a *cachedAccessor) BlockByHeight(height uint64) (types.BlockDetail, error) {
	res, err := a.getOrLoad("BlockByHeight", func() (interface{}, error) {
		return a.accessor.BlockByHeight(height)
	}, height)
	return res.(types.BlockDetail), err
}

func (a *cachedAccessor) BlockTxsCountByHeight(height uint64) (uint64, error) {
	res, err := a.getOrLoad("BlockTxsCountByHeight", func() (interface{}, error) {
		return a.accessor.BlockTxsCountByHeight(height)
	}, height)
	return res.(uint64), err
}

func (a *cachedAccessor) BlockTxsByHeight(height uint64, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("BlockTxsByHeight", func() (interface{}, *string, error) {
		return a.accessor.BlockTxsByHeight(height, count, continuationToken)
	}, height, count, continuationToken)
	return res.([]types.TransactionSummary), nextContinuationToken, err
}

func (a *cachedAccessor) BlockByHash(hash string) (types.BlockDetail, error) {
	res, err := a.getOrLoad("BlockByHash", func() (interface{}, error) {
		return a.accessor.BlockByHash(hash)
	}, hash)
	return res.(types.BlockDetail), err
}

func (a *cachedAccessor) BlockTxsCountByHash(hash string) (uint64, error) {
	res, err := a.getOrLoad("BlockTxsCountByHash", func() (interface{}, error) {
		return a.accessor.BlockTxsCountByHash(hash)
	}, hash)
	return res.(uint64), err
}

func (a *cachedAccessor) BlockTxsByHash(hash string, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("BlockTxsByHash", func() (interface{}, *string, error) {
		return a.accessor.BlockTxsByHash(hash, count, continuationToken)
	}, hash, count, continuationToken)
	return res.([]types.TransactionSummary), nextContinuationToken, err
}

func (a *cachedAccessor) BlockCoinsByHeight(height uint64) (types.AllCoins, error) {
	res, err := a.getOrLoad("BlockCoinsByHeight", func() (interface{}, error) {
		return a.accessor.BlockCoinsByHeight(height)
	}, height)
	return res.(types.AllCoins), err
}

func (a *cachedAccessor) BlockCoinsByHash(hash string) (types.AllCoins, error) {
	res, err := a.getOrLoad("BlockCoinsByHash", func() (interface{}, error) {
		return a.accessor.BlockCoinsByHash(hash)
	}, hash)
	return res.(types.AllCoins), err
}

func (a *cachedAccessor) LastBlock() (types.BlockDetail, error) {
	res, err := a.getOrLoad(lastBlock, func() (interface{}, error) {
		return a.accessor.LastBlock()
	})
	return res.(types.BlockDetail), err
}

func (a *cachedAccessor) Flip(hash string) (types.Flip, error) {
	res, err := a.getOrLoad("Flip", func() (interface{}, error) {
		return a.accessor.Flip(hash)
	}, hash)
	return res.(types.Flip), err
}

func (a *cachedAccessor) FlipContent(hash string) (types.FlipContent, error) {
	res, err := a.getOrLoad("FlipContent", func() (interface{}, error) {
		return a.accessor.FlipContent(hash)
	}, hash)
	return res.(types.FlipContent), err
}

func (a *cachedAccessor) FlipAnswers(hash string, isShort bool) ([]types.Answer, error) {
	res, err := a.getOrLoad("FlipAnswers", func() (interface{}, error) {
		return a.accessor.FlipAnswers(hash, isShort)
	}, hash, isShort)
	return res.([]types.Answer), err
}

func (a *cachedAccessor) FlipEpochAdjacentFlips(hash string) (types.AdjacentStrValues, error) {
	res, err := a.getOrLoad(flipEpochAdjacentFlipsMethod, func() (interface{}, error) {
		return a.accessor.FlipEpochAdjacentFlips(hash)
	}, hash)
	return res.(types.AdjacentStrValues), err
}

func (a *cachedAccessor) Identity(address string) (types.Identity, error) {
	res, err := a.getOrLoad("Identity", func() (interface{}, error) {
		return a.accessor.Identity(address)
	}, address)
	return res.(types.Identity), err
}

func (a *cachedAccessor) IdentityAge(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityAge", func() (interface{}, error) {
		return a.accessor.IdentityAge(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityCurrentFlipCids(address string) ([]string, error) {
	res, err := a.getOrLoad("IdentityCurrentFlipCids", func() (interface{}, error) {
		return a.accessor.IdentityCurrentFlipCids(address)
	}, address)
	return res.([]string), err
}

func (a *cachedAccessor) IdentityEpochsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityEpochsCount", func() (interface{}, error) {
		return a.accessor.IdentityEpochsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityEpochs(address string, count uint64, continuationToken *string) ([]types.EpochIdentity, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("IdentityEpochs", func() (interface{}, *string, error) {
		return a.accessor.IdentityEpochs(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.EpochIdentity), nextContinuationToken, err
}

func (a *cachedAccessor) IdentityFlipsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityFlipsCount", func() (interface{}, error) {
		return a.accessor.IdentityFlipsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityFlips(address string, count uint64, continuationToken *string) ([]types.FlipSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("IdentityFlips", func() (interface{}, *string, error) {
		return a.accessor.IdentityFlips(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.FlipSummary), nextContinuationToken, err
}

func (a *cachedAccessor) IdentityFlipQualifiedAnswers(address string) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad("IdentityFlipQualifiedAnswers", func() (interface{}, error) {
		return a.accessor.IdentityFlipQualifiedAnswers(address)
	}, address)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) IdentityFlipStates(address string) ([]types.StrValueCount, error) {
	res, err := a.getOrLoad("IdentityFlipStates", func() (interface{}, error) {
		return a.accessor.IdentityFlipStates(address)
	}, address)
	return res.([]types.StrValueCount), err
}

func (a *cachedAccessor) IdentityInvitesCount(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityInvitesCount", func() (interface{}, error) {
		return a.accessor.IdentityInvitesCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityInvites(address string, count uint64, continuationToken *string) ([]types.Invite, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("IdentityInvites", func() (interface{}, *string, error) {
		return a.accessor.IdentityInvites(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.Invite), nextContinuationToken, err
}

func (a *cachedAccessor) IdentityTxsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityTxsCount", func() (interface{}, error) {
		return a.accessor.IdentityTxsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityTxs(address string, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("IdentityTxs", func() (interface{}, *string, error) {
		return a.accessor.IdentityTxs(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.TransactionSummary), nextContinuationToken, err
}

func (a *cachedAccessor) IdentityRewardsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityRewardsCount", func() (interface{}, error) {
		return a.accessor.IdentityRewardsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityRewards(address string, count uint64, continuationToken *string) ([]types.Reward, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("IdentityRewards", func() (interface{}, *string, error) {
		return a.accessor.IdentityRewards(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.Reward), nextContinuationToken, err
}

func (a *cachedAccessor) IdentityEpochRewardsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("IdentityEpochRewardsCount", func() (interface{}, error) {
		return a.accessor.IdentityEpochRewardsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) IdentityEpochRewards(address string, count uint64, continuationToken *string) ([]types.Rewards, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("IdentityEpochRewards", func() (interface{}, *string, error) {
		return a.accessor.IdentityEpochRewards(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.Rewards), nextContinuationToken, err
}

func (a *cachedAccessor) Address(address string) (types.Address, error) {
	res, err := a.getOrLoad("Address", func() (interface{}, error) {
		return a.accessor.Address(address)
	}, address)
	return res.(types.Address), err
}

func (a *cachedAccessor) AddressPenaltiesCount(address string) (uint64, error) {
	res, err := a.getOrLoad("AddressPenaltiesCount", func() (interface{}, error) {
		return a.accessor.AddressPenaltiesCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) AddressPenalties(address string, count uint64, continuationToken *string) ([]types.Penalty, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressPenalties", func() (interface{}, *string, error) {
		return a.accessor.AddressPenalties(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.Penalty), nextContinuationToken, err
}

func (a *cachedAccessor) AddressStatesCount(address string) (uint64, error) {
	res, err := a.getOrLoad("AddressStatesCount", func() (interface{}, error) {
		return a.accessor.AddressStatesCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) AddressStates(address string, count uint64, continuationToken *string) ([]types.AddressState, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressStates", func() (interface{}, *string, error) {
		return a.accessor.AddressStates(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.AddressState), nextContinuationToken, err
}

func (a *cachedAccessor) AddressTotalLatestMiningReward(afterTime time.Time, address string) (types.TotalMiningReward, error) {
	res, err := a.getOrLoad("AddressTotalLatestMiningReward", func() (interface{}, error) {
		return a.accessor.AddressTotalLatestMiningReward(afterTime, address)
	}, afterTime, address)
	return res.(types.TotalMiningReward), err
}

func (a *cachedAccessor) AddressTotalLatestBurntCoins(afterTime time.Time, address string) (types.AddressBurntCoins, error) {
	res, err := a.getOrLoad("AddressTotalLatestBurntCoins", func() (interface{}, error) {
		return a.accessor.AddressTotalLatestBurntCoins(afterTime, address)
	}, afterTime, address)
	return res.(types.AddressBurntCoins), err
}

func (a *cachedAccessor) AddressBadAuthorsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("AddressBadAuthorsCount", func() (interface{}, error) {
		return a.accessor.AddressBadAuthorsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) AddressBadAuthors(address string, count uint64, continuationToken *string) ([]types.BadAuthor, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressBadAuthors", func() (interface{}, *string, error) {
		return a.accessor.AddressBadAuthors(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.BadAuthor), nextContinuationToken, err
}

func (a *cachedAccessor) AddressBalanceUpdatesCount(address string) (uint64, error) {
	res, err := a.getOrLoad("AddressBalanceUpdatesCount", func() (interface{}, error) {
		return a.accessor.AddressBalanceUpdatesCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) AddressBalanceUpdates(address string, count uint64, continuationToken *string) ([]types.BalanceUpdate, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressBalanceUpdates", func() (interface{}, *string, error) {
		return a.accessor.AddressBalanceUpdates(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.BalanceUpdate), nextContinuationToken, err
}

func (a *cachedAccessor) AddressBalanceUpdatesSummary(address string) (*types.BalanceUpdatesSummary, error) {
	res, err := a.getOrLoad("AddressBalanceUpdatesSummary", func() (interface{}, error) {
		return a.accessor.AddressBalanceUpdatesSummary(address)
	}, address)
	return res.(*types.BalanceUpdatesSummary), err
}

func (a *cachedAccessor) AddressDelegateeTotalRewards(address string, count uint64, continuationToken *string) ([]types.DelegateeTotalRewards, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressDelegateeTotalRewards", func() (interface{}, *string, error) {
		return a.accessor.AddressDelegateeTotalRewards(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.DelegateeTotalRewards), nextContinuationToken, err
}

func (a *cachedAccessor) AddressMiningRewardSummaries(address string, count uint64, continuationToken *string) ([]types.MiningRewardSummary, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressMiningRewardSummaries", func() (interface{}, *string, error) {
		return a.accessor.AddressMiningRewardSummaries(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.MiningRewardSummary), nextContinuationToken, err
}

func (a *cachedAccessor) AddressTokens(address string, count uint64, continuationToken *string) ([]types.TokenBalance, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressTokens", func() (interface{}, *string, error) {
		return a.accessor.AddressTokens(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.TokenBalance), nextContinuationToken, err
}

func (a *cachedAccessor) AddressToken(address, tokenAddress string) (types.TokenBalance, error) {
	res, err := a.getOrLoad("AddressToken", func() (interface{}, error) {
		return a.accessor.AddressToken(address, tokenAddress)
	}, address, tokenAddress)
	return res.(types.TokenBalance), err
}

func (a *cachedAccessor) AddressDelegations(address string, count uint64, continuationToken *string) ([]types.Delegation, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressDelegations", func() (interface{}, *string, error) {
		return a.accessor.AddressDelegations(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.Delegation), nextContinuationToken, err
}

func (a *cachedAccessor) Transaction(hash string) (*types.TransactionDetail, error) {
	res, err := a.getOrLoad("Transaction", func() (interface{}, error) {
		tx, err := a.accessor.Transaction(hash)
		if err == postgres.NoDataFound {
			tx, err = a.memPool.GetTransaction(hash)
		}
		return tx, err
	}, hash)
	return res.(*types.TransactionDetail), err
}

func (a *cachedAccessor) TransactionRaw(hash string) (*hexutil.Bytes, error) {
	res, err := a.getOrLoad("TransactionRaw", func() (interface{}, error) {
		txRaw, err := a.accessor.TransactionRaw(hash)
		if err == postgres.NoDataFound {
			txRaw, err = a.memPool.GetTransactionRaw(hash)
		}
		return txRaw, err
	}, hash)
	return res.(*hexutil.Bytes), err
}

func (a *cachedAccessor) TransactionEvents(hash string, count uint64, continuationToken *string) ([]types.TxEvent, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("TransactionEvents", func() (interface{}, *string, error) {
		return a.accessor.TransactionEvents(hash, count, continuationToken)
	}, hash, count, continuationToken)
	return res.([]types.TxEvent), nextContinuationToken, err
}

func (a *cachedAccessor) BalancesCount() (uint64, error) {
	res, err := a.getOrLoad("BalancesCount", func() (interface{}, error) {
		return a.accessor.BalancesCount()
	})
	return res.(uint64), err
}

func (a *cachedAccessor) Balances(sortBy *string, count uint64, continuationToken *string) ([]types.Balance, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("Balances", func() (interface{}, *string, error) {
		return a.accessor.Balances(sortBy, count, continuationToken)
	}, sortBy, count, continuationToken)
	return res.([]types.Balance), nextContinuationToken, err
}

func (a *cachedAccessor) TotalLatestMiningRewardsCount(afterTime time.Time) (uint64, error) {
	res, err := a.getOrLoad("TotalLatestMiningRewardsCount", func() (interface{}, error) {
		return a.accessor.TotalLatestMiningRewardsCount(afterTime)
	}, afterTime)
	return res.(uint64), err
}

func (a *cachedAccessor) TotalLatestMiningRewards(afterTime time.Time, startIndex uint64,
	count uint64) ([]types.TotalMiningReward, error) {
	res, err := a.getOrLoad("TotalLatestMiningRewards", func() (interface{}, error) {
		return a.accessor.TotalLatestMiningRewards(afterTime, startIndex, count)
	}, afterTime, startIndex, count)
	return res.([]types.TotalMiningReward), err
}

func (a *cachedAccessor) TotalLatestBurntCoinsCount(afterTime time.Time) (uint64, error) {
	res, err := a.getOrLoad("TotalLatestBurntCoinsCount", func() (interface{}, error) {
		return a.accessor.TotalLatestBurntCoinsCount(afterTime)
	}, afterTime)
	return res.(uint64), err
}

func (a *cachedAccessor) TotalLatestBurntCoins(afterTime time.Time, startIndex uint64,
	count uint64) ([]types.AddressBurntCoins, error) {
	res, err := a.getOrLoad("TotalLatestBurntCoins", func() (interface{}, error) {
		return a.accessor.TotalLatestBurntCoins(afterTime, startIndex, count)
	}, afterTime, startIndex, count)
	return res.([]types.AddressBurntCoins), err
}

func (a *cachedAccessor) Contract(address string) (types.Contract, error) {
	res, err := a.getOrLoad("Contract", func() (interface{}, error) {
		return a.accessor.Contract(address)
	}, address)
	return res.(types.Contract), err
}

func (a *cachedAccessor) ContractTxBalanceUpdates(contractAddress string, count uint64, continuationToken *string) ([]types.ContractTxBalanceUpdate, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("ContractTxBalanceUpdates", func() (interface{}, *string, error) {
		return a.accessor.ContractTxBalanceUpdates(contractAddress, count, continuationToken)
	}, contractAddress, count, continuationToken)
	return res.([]types.ContractTxBalanceUpdate), nextContinuationToken, err
}

func (a *cachedAccessor) ContractVerifiedCodeFile(address string) ([]byte, error) {
	res, err := a.getOrLoad("ContractVerifiedCodeFile", func() (interface{}, error) {
		return a.accessor.ContractVerifiedCodeFile(address)
	}, address)
	return res.([]byte), err
}

func (a *cachedAccessor) TimeLockContract(address string) (types.TimeLockContract, error) {
	res, err := a.getOrLoad("TimeLockContract", func() (interface{}, error) {
		return a.accessor.TimeLockContract(address)
	}, address)
	return res.(types.TimeLockContract), err
}

func (a *cachedAccessor) MultisigContract(address string) (types.MultisigContract, error) {
	res, err := a.getOrLoad("MultisigContract", func() (interface{}, error) {
		return a.accessor.MultisigContract(address)
	}, address)
	return res.(types.MultisigContract), err
}

func (a *cachedAccessor) OracleLockContract(address string) (types.OracleLockContract, error) {
	res, err := a.getOrLoad("OracleLockContract", func() (interface{}, error) {
		return a.accessor.OracleLockContract(address)
	}, address)
	return res.(types.OracleLockContract), err
}

func (a *cachedAccessor) RefundableOracleLockContract(address string) (types.RefundableOracleLockContract, error) {
	res, err := a.getOrLoad("RefundableOracleLockContract", func() (interface{}, error) {
		return a.accessor.RefundableOracleLockContract(address)
	}, address)
	return res.(types.RefundableOracleLockContract), err
}

func (a *cachedAccessor) OracleVotingContracts(authorAddress, oracleAddress string, states []string, all bool, sortBy *string, count uint64, continuationToken *string) ([]types.OracleVotingContract, *string, error) {
	return a.accessor.OracleVotingContracts(authorAddress, oracleAddress, states, all, sortBy, count, continuationToken)
}

func (a *cachedAccessor) AddressOracleVotingContracts(address string, count uint64, continuationToken *string) ([]types.OracleVotingContract, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("AddressOracleVotingContracts", func() (interface{}, *string, error) {
		return a.accessor.AddressOracleVotingContracts(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.OracleVotingContract), nextContinuationToken, err
}

func (a *cachedAccessor) OracleVotingContract(address, oracle string) (types.OracleVotingContract, error) {
	return a.accessor.OracleVotingContract(address, oracle)
}

func (a *cachedAccessor) EstimatedOracleRewards() ([]types.EstimatedOracleReward, error) {
	return a.accessor.EstimatedOracleRewards()
}

func (a *cachedAccessor) AddressContractTxBalanceUpdates(address, contractAddress string, count uint64, continuationToken *string) ([]types.ContractTxBalanceUpdate, *string, error) {
	return a.accessor.AddressContractTxBalanceUpdates(address, contractAddress, count, continuationToken)
}

func (a *cachedAccessor) Upgrades(count uint64, continuationToken *string) ([]types.ActivatedUpgrade, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("Upgrades", func() (interface{}, *string, error) {
		return a.accessor.Upgrades(count, continuationToken)
	}, count, continuationToken)
	return res.([]types.ActivatedUpgrade), nextContinuationToken, err
}

func (a *cachedAccessor) UpgradeVotings(count uint64, continuationToken *string) ([]types.Upgrade, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("UpgradeVotings", func() (interface{}, *string, error) {
		return a.accessor.UpgradeVotings(count, continuationToken)
	}, count, continuationToken)
	return res.([]types.Upgrade), nextContinuationToken, err
}

func (a *cachedAccessor) UpgradeVotingHistory(upgrade uint64) ([]*types.UpgradeVotingHistoryItem, error) {
	res, err := a.getOrLoad("UpgradeVotingHistory", func() (interface{}, error) {
		return a.accessor.UpgradeVotingHistory(upgrade)
	}, upgrade)
	return res.([]*types.UpgradeVotingHistoryItem), err
}

func (a *cachedAccessor) Upgrade(upgrade uint64) (*types.Upgrade, error) {
	res, err := a.getOrLoad(upgradeMethod, func() (interface{}, error) {
		return a.accessor.Upgrade(upgrade)
	}, upgrade)
	return res.(*types.Upgrade), err
}

func (a *cachedAccessor) PoolsCount() (uint64, error) {
	res, err := a.getOrLoad("PoolsCount", func() (interface{}, error) {
		return a.accessor.PoolsCount()
	})
	return res.(uint64), err
}

func (a *cachedAccessor) Pools(count uint64, continuationToken *string) ([]*types.Pool, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("Pools", func() (interface{}, *string, error) {
		return a.accessor.Pools(count, continuationToken)
	}, count, continuationToken)
	return res.([]*types.Pool), nextContinuationToken, err
}

func (a *cachedAccessor) Pool(address string) (*types.Pool, error) {
	res, err := a.getOrLoad("Pool", func() (interface{}, error) {
		return a.accessor.Pool(address)
	}, address)
	return res.(*types.Pool), err
}

func (a *cachedAccessor) PoolDelegatorsCount(address string) (uint64, error) {
	res, err := a.getOrLoad("PoolDelegatorsCount", func() (interface{}, error) {
		return a.accessor.PoolDelegatorsCount(address)
	}, address)
	return res.(uint64), err
}

func (a *cachedAccessor) PoolDelegators(address string, count uint64, continuationToken *string) ([]*types.Delegator, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("PoolDelegators", func() (interface{}, *string, error) {
		return a.accessor.PoolDelegators(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]*types.Delegator), nextContinuationToken, err
}

func (a *cachedAccessor) PoolSizeHistory(address string, count uint64, continuationToken *string) ([]types.PoolSizeHistoryItem, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("PoolSizeHistory", func() (interface{}, *string, error) {
		return a.accessor.PoolSizeHistory(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.PoolSizeHistoryItem), nextContinuationToken, err
}

func (a *cachedAccessor) MinersHistory() ([]types.MinersHistoryItem, error) {
	res, err := a.getOrLoad("MinersHistory", func() (interface{}, error) {
		return a.accessor.MinersHistory()
	})
	return res.([]types.MinersHistoryItem), err
}

func (a *cachedAccessor) PeersHistory(count uint64) ([]types.PeersHistoryItem, error) {
	res, err := a.getOrLoad("PeersHistory", func() (interface{}, error) {
		return a.accessor.PeersHistory(count)
	}, count)
	return res.([]types.PeersHistoryItem), err
}

func (a *cachedAccessor) DynamicEndpoints() ([]types.DynamicEndpoint, error) {
	return a.accessor.DynamicEndpoints()
}

func (a *cachedAccessor) DynamicEndpointData(name string, limit *int) (*types.DynamicEndpointResult, error) {
	res, err := a.getOrLoad("DynamicEndpointData", func() (interface{}, error) {
		return a.accessor.DynamicEndpointData(name, limit)
	}, name, limit)
	return res.(*types.DynamicEndpointResult), err
}

func (a *cachedAccessor) Token(address string) (types.Token, error) {
	res, err := a.getOrLoad("Token", func() (interface{}, error) {
		return a.accessor.Token(address)
	}, address)
	return res.(types.Token), err
}

func (a *cachedAccessor) TokenHolders(address string, count uint64, continuationToken *string) ([]types.TokenBalance, *string, error) {
	res, nextContinuationToken, err := a.getOrLoadWithConToken("TokenHolders", func() (interface{}, *string, error) {
		return a.accessor.TokenHolders(address, count, continuationToken)
	}, address, count, continuationToken)
	return res.([]types.TokenBalance), nextContinuationToken, err
}

func (a *cachedAccessor) Destroy() {
	a.accessor.Destroy()
}
