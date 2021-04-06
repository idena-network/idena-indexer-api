package indexer

import (
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/idena-network/idena-indexer-api/log"
	"github.com/pkg/errors"
	"strconv"
)

type Api interface {
	OnlineIdentitiesCount() (uint64, error)
	OnlineIdentities(count uint64, continuationToken *string) ([]*types.OnlineIdentity, *string, error)
	OnlineIdentitiesOld(startIndex, count uint64) ([]*types.OnlineIdentity, error)
	OnlineIdentity(address string) (*types.OnlineIdentity, error)
	OnlineCount() (uint64, error)

	MemPoolTransaction(hash string) (*types.TransactionDetail, error)
	MemPoolTransactionRaw(hash string) (*hexutil.Bytes, error)
	MemPoolAddressTransactions(address string, count int) ([]*types.TransactionSummary, error)
	MemPoolTransactions(count int) ([]*types.TransactionSummary, error)

	MemPoolOracleVotingContractDeploys(author string) ([]service.OracleVotingContract, error)
	MemPoolAddressContractTxs(address, contractAddress string) ([]service.Transaction, error)

	UpgradeVoting() ([]*types.UpgradeVotes, error)
}

func NewApi(client Client, logger log.Logger) Api {
	res := &apiImpl{
		client: client,
		logger: logger,
	}
	return res
}

type apiImpl struct {
	client Client
	logger log.Logger
}

func (api *apiImpl) OnlineIdentitiesCount() (uint64, error) {
	var res uint64
	_, _, err := api.client.Get("api/OnlineIdentities/Count", &res)
	if err != nil {
		return 0, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) OnlineIdentities(count uint64, continuationToken *string) ([]*types.OnlineIdentity, *string, error) {
	var optional string
	if continuationToken != nil {
		optional = "&continuationToken=" + *continuationToken
	}
	var res []*types.OnlineIdentity
	_, nextContinuationToken, err := api.client.Get("api/OnlineIdentities?limit="+strconv.Itoa(int(count))+optional, &res)
	if err != nil {
		return nil, nil, api.handleError(err)
	}
	return res, nextContinuationToken, nil
}

func (api *apiImpl) OnlineIdentitiesOld(startIndex, count uint64) ([]*types.OnlineIdentity, error) {
	var res []*types.OnlineIdentity
	_, _, err := api.client.Get("api/OnlineIdentities?limit="+strconv.Itoa(int(count))+"&skip="+strconv.Itoa(int(startIndex)), &res)
	if err != nil {
		return nil, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) OnlineIdentity(address string) (*types.OnlineIdentity, error) {
	res, _, err := api.client.Get("api/OnlineIdentity/"+address, &types.OnlineIdentity{})
	if err != nil || res == nil {
		return nil, api.handleError(err)
	}
	return res.(*types.OnlineIdentity), nil
}

func (api *apiImpl) OnlineCount() (uint64, error) {
	var res uint64
	_, _, err := api.client.Get("api/OnlineMiners/Count", &res)
	if err != nil {
		return 0, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) MemPoolTransaction(hash string) (*types.TransactionDetail, error) {
	res, _, err := api.client.Get("api/MemPool/Transaction/"+hash, &types.TransactionDetail{})
	if err != nil || res == nil {
		return nil, api.handleError(err)
	}
	return res.(*types.TransactionDetail), nil
}

func (api *apiImpl) MemPoolTransactionRaw(hash string) (*hexutil.Bytes, error) {
	res, _, err := api.client.Get("api/MemPool/Transaction/"+hash+"/Raw", &hexutil.Bytes{})
	if err != nil || res == nil {
		return nil, api.handleError(err)
	}
	return res.(*hexutil.Bytes), nil
}

func (api *apiImpl) MemPoolAddressTransactions(address string, count int) ([]*types.TransactionSummary, error) {
	var res []*types.TransactionSummary
	_, _, err := api.client.Get("api/MemPool/Address/"+address+"/Transactions?limit="+strconv.Itoa(count), &res)
	if err != nil {
		return nil, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) MemPoolTransactions(count int) ([]*types.TransactionSummary, error) {
	var res []*types.TransactionSummary
	_, _, err := api.client.Get("api/MemPool/Transactions?limit="+strconv.Itoa(count), &res)
	if err != nil {
		return nil, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) MemPoolOracleVotingContractDeploys(author string) ([]service.OracleVotingContract, error) {
	var res []service.OracleVotingContract
	_, _, err := api.client.Get("api/MemPool/OracleVotingContractDeploys?author="+author, &res)
	if err != nil {
		return nil, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) MemPoolAddressContractTxs(address, contractAddress string) ([]service.Transaction, error) {
	var res []service.Transaction
	_, _, err := api.client.Get("api/MemPool/Address/"+address+"/Contract/"+contractAddress+"/Txs", &res)
	if err != nil {
		return nil, api.handleError(err)
	}
	return res, nil
}

func (api *apiImpl) UpgradeVoting() (res []*types.UpgradeVotes, err error) {
	if _, _, err = api.client.Get("api/UpgradeVoting", &res); err != nil {
		return nil, api.handleError(err)
	}
	return res, nil
}

var indexerError = errors.New("unable to load indexer data")

func (api *apiImpl) handleError(err error) error {
	if err == nil {
		return nil
	}
	api.logger.Error(err.Error())
	return indexerError
}
