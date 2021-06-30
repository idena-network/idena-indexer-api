package api

import (
	"fmt"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-indexer-api/app/db"
	"github.com/idena-network/idena-indexer-api/app/db/postgres"
	service2 "github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/idena-network/idena-indexer-api/indexer"
)

type Service interface {
	db.Accessor

	MemPoolTxs(count uint64) ([]*types.TransactionSummary, error)
	MemPoolTxsCount() (int, error)
	GetOnlineIdentitiesCount() (uint64, error)
	GetOnlineIdentities(count uint64, continuationToken *string) ([]*types.OnlineIdentity, *string, error)
	GetOnlineIdentitiesOld(startIndex, count uint64) ([]*types.OnlineIdentity, error)
	GetOnlineIdentity(address string) (*types.OnlineIdentity, error)
	GetOnlineCount() (uint64, error)
	ValidatorsCount() (uint64, error)
	Validators(count uint64, continuationToken *string) ([]types.Validator, *string, error)
	OnlineValidatorsCount() (uint64, error)
	OnlineValidators(count uint64, continuationToken *string) ([]types.Validator, *string, error)
	SignatureAddress(value, signature string) (string, error)
	UpgradeVoting() ([]*types.UpgradeVotes, error)

	ForkChangeLog(version string) (*service2.ChangeLogData, error)
}

type MemPool interface {
	GetTransaction(hash string) (*types.TransactionDetail, error)
	GetTransactionRaw(hash string) (*hexutil.Bytes, error)
	GetAddressTransactions(address string, count int) ([]*types.TransactionSummary, error)
	GetTransactions(count int) ([]*types.TransactionSummary, error)
	GetTransactionsCount() (int, error)
}

func NewService(dbAccessor db.Accessor, memPool MemPool, indexerApi indexer.Api, changeLog service2.ChangeLog) Service {
	return &service{
		Accessor:   dbAccessor,
		memPool:    memPool,
		indexerApi: indexerApi,
		changeLog:  changeLog,
	}
}

type service struct {
	db.Accessor
	memPool    MemPool
	indexerApi indexer.Api
	changeLog  service2.ChangeLog
}

func (s *service) Search(value string) ([]types.Entity, error) {
	res, err := s.Accessor.Search(value)
	if err != nil {
		return nil, err
	}
	tx, _ := s.memPool.GetTransaction(value)
	if tx != nil {
		res = append(res, types.Entity{
			Name:     "Transaction",
			Value:    value,
			Ref:      fmt.Sprintf("/api/Transaction/%s", value),
			NameOld:  "Transaction",
			ValueOld: value,
			RefOld:   fmt.Sprintf("/api/Transaction/%s", value),
		})
	}
	return res, err
}

func (s *service) Transaction(hash string) (*types.TransactionDetail, error) {
	res, err := s.Accessor.Transaction(hash)
	if err != postgres.NoDataFound {
		return res, err
	}

	res, err = s.memPool.GetTransaction(hash)
	if err == nil && res == nil {
		err = postgres.NoDataFound
	}
	return res, err
}

func (s *service) TransactionRaw(hash string) (*hexutil.Bytes, error) {
	res, err := s.Accessor.TransactionRaw(hash)
	if err != postgres.NoDataFound {
		return res, err
	}

	res, err = s.memPool.GetTransactionRaw(hash)
	if err == nil && res == nil {
		err = postgres.NoDataFound
	}
	return res, err
}

func (s *service) IdentityTxs(address string, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	var res []types.TransactionSummary
	var nextContinuationToken *string
	var err error
	if continuationToken == nil {
		// Mem pool txs
		txs, _ := s.memPool.GetAddressTransactions(address, int(count))
		if len(txs) > 0 {
			res = make([]types.TransactionSummary, 0, len(txs))
			for _, tx := range txs {
				res = append(res, *tx)
				if len(res) == int(count) {
					break
				}
			}
			count = count - uint64(len(res))
		}
	}

	if count > 0 {
		// DB txs
		var txs []types.TransactionSummary
		txs, nextContinuationToken, err = s.Accessor.IdentityTxs(address, count, continuationToken)
		res = append(res, txs...)
	}
	return res, nextContinuationToken, err
}

func (s *service) MemPoolTxs(count uint64) ([]*types.TransactionSummary, error) {
	return s.memPool.GetTransactions(int(count))
}

func (s *service) MemPoolTxsCount() (int, error) {
	return s.memPool.GetTransactionsCount()
}

func (s *service) GetOnlineIdentitiesCount() (uint64, error) {
	return s.indexerApi.OnlineIdentitiesCount()
}

func (s *service) GetOnlineIdentities(count uint64, continuationToken *string) ([]*types.OnlineIdentity, *string, error) {
	return s.indexerApi.OnlineIdentities(count, continuationToken)
}

// Deprecated
func (s *service) GetOnlineIdentitiesOld(startIndex, count uint64) ([]*types.OnlineIdentity, error) {
	return s.indexerApi.OnlineIdentitiesOld(startIndex, count)
}

func (s *service) GetOnlineIdentity(address string) (*types.OnlineIdentity, error) {
	return s.indexerApi.OnlineIdentity(address)
}

func (s *service) GetOnlineCount() (uint64, error) {
	return s.indexerApi.OnlineCount()
}

func (s *service) ValidatorsCount() (uint64, error) {
	return s.indexerApi.ValidatorsCount()
}

func (s *service) Validators(count uint64, continuationToken *string) ([]types.Validator, *string, error) {
	return s.indexerApi.Validators(count, continuationToken)
}

func (s *service) OnlineValidatorsCount() (uint64, error) {
	return s.indexerApi.OnlineValidatorsCount()
}

func (s *service) OnlineValidators(count uint64, continuationToken *string) ([]types.Validator, *string, error) {
	return s.indexerApi.OnlineValidators(count, continuationToken)
}

func (s *service) SignatureAddress(value, signature string) (string, error) {
	hash := crypto.Hash([]byte(value))
	hash = crypto.Hash(hash[:])
	signatureBytes, err := hexutil.Decode(signature)
	if err != nil {
		return "", err
	}
	pubKey, err := crypto.Ecrecover(hash[:], signatureBytes)
	if err != nil {
		return "", err
	}
	addr, err := crypto.PubKeyBytesToAddress(pubKey)
	if err != nil {
		return "", err
	}
	return addr.Hex(), nil
}

func (s *service) UpgradeVoting() ([]*types.UpgradeVotes, error) {
	return s.indexerApi.UpgradeVoting()
}

func (s *service) ForkChangeLog(version string) (*service2.ChangeLogData, error) {
	return s.changeLog.ForkChangeLog(version)
}
