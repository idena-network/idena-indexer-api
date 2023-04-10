package api

import (
	"fmt"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-indexer-api/app/db"
	service2 "github.com/idena-network/idena-indexer-api/app/service"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/idena-network/idena-indexer-api/indexer"
	"github.com/shopspring/decimal"
)

type Service interface {
	db.Accessor

	MemPoolTxs(count uint64) ([]*types.TransactionSummary, error)
	MemPoolTxsCount() (int, error)
	GetOnlineIdentitiesCount() (uint64, error)
	GetOnlineIdentities(count uint64, continuationToken *string) ([]*types.OnlineIdentity, *string, error)
	GetOnlineIdentity(address string) (*types.OnlineIdentity, error)
	GetOnlineCount() (uint64, error)
	ValidatorsCount() (uint64, error)
	Validators(count uint64, continuationToken *string) ([]types.Validator, *string, error)
	OnlineValidatorsCount() (uint64, error)
	OnlineValidators(count uint64, continuationToken *string) ([]types.Validator, *string, error)
	ForkCommitteeCount() (uint64, error)
	SignatureAddress(value, signature string) (string, error)
	UpgradeVoting() ([]*types.UpgradeVotes, error)
	Staking() (*types.Staking, error)

	IdentityWithProof(address string, height uint64) (*hexutil.Bytes, error)

	ForkChangeLog(version string) (*service2.ChangeLogData, error)

	Upgrades(count uint64, continuationToken *string) ([]types.ActivatedUpgrade, *string, error)
	UpgradeVotings(count uint64, continuationToken *string) ([]types.Upgrade, *string, error)
	Upgrade(upgrade uint64) (*types.Upgrade, error)

	VerifyContract(address string, data []byte) error
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
	return s.Accessor.Transaction(hash)
}

func (s *service) TransactionRaw(hash string) (*hexutil.Bytes, error) {
	return s.Accessor.TransactionRaw(hash)
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

func (s *service) ForkCommitteeCount() (uint64, error) {
	return s.indexerApi.ForkCommitteeCount()
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

func (s *service) Upgrades(count uint64, continuationToken *string) ([]types.ActivatedUpgrade, *string, error) {
	upgrades, nextContinuationToken, err := s.Accessor.Upgrades(count, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	for i, upgrade := range upgrades {
		if upgrade.Upgrade != nil {
			upgrades[i].Url = s.changeLog.Url(*upgrade.Upgrade)
		}
	}
	return upgrades, nextContinuationToken, nil
}

func (s *service) UpgradeVotings(count uint64, continuationToken *string) ([]types.Upgrade, *string, error) {
	upgrades, nextContinuationToken, err := s.Accessor.UpgradeVotings(count, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	for i, upgrade := range upgrades {
		upgrades[i].Url = s.changeLog.Url(upgrade.Upgrade)
	}
	return upgrades, nextContinuationToken, nil
}

func (s *service) Upgrade(upgrade uint64) (*types.Upgrade, error) {
	res, err := s.Accessor.Upgrade(upgrade)
	if err != nil {
		return nil, err
	}
	res.Url = s.changeLog.Url(res.Upgrade)
	return res, nil
}

func (s *service) IdentityWithProof(address string, epoch uint64) (*hexutil.Bytes, error) {
	return s.indexerApi.IdentityWithProof(epoch, address)
}

func (s *service) Staking() (*types.Staking, error) {
	staking, err := s.indexerApi.Staking()
	if err != nil {
		return nil, err
	}
	lastEpoch, err := s.Accessor.LastEpoch()
	if err != nil {
		return nil, err
	}
	if lastEpoch.Epoch == 0 {
		return &types.Staking{}, nil
	}
	rewardsSummary, err := s.Accessor.EpochRewardsSummary(lastEpoch.Epoch - 1)
	if err != nil {
		return nil, err
	}

	calculateWeight := func(totalReward, rewardShare decimal.Decimal) float64 {
		if rewardShare.IsZero() {
			return 0
		}
		res, _ := totalReward.Div(rewardShare).Float64()
		return res
	}

	staking.ExtraFlipsWeight = calculateWeight(rewardsSummary.ExtraFlips, rewardsSummary.ExtraFlipsShare)
	staking.InvitationsWeight = calculateWeight(rewardsSummary.Invitations, rewardsSummary.InvitationsShare)

	return staking, nil
}

func (s *service) MultisigContract(address string) (types.MultisigContract, error) {
	res, err := s.Accessor.MultisigContract(address)
	if err != nil {
		return types.MultisigContract{}, err
	}
	indexerContract, err := s.indexerApi.MultisigContract(address)
	if err != nil {
		return types.MultisigContract{}, err
	}
	res.Signers = indexerContract.Signers
	return res, nil
}

func (s *service) Pool(address string) (*types.Pool, error) {
	res, err := s.Accessor.Pool(address)
	if err != nil {
		return nil, err
	}
	indexerPool, err := s.indexerApi.Pool(address)
	if err != nil {
		return nil, err
	}
	res.TotalStake = indexerPool.TotalStake
	res.TotalValidatedStake = indexerPool.TotalValidatedStake
	return res, nil
}

func (s *service) VerifyContract(address string, data []byte) error {
	return s.indexerApi.VerifyContract(address, data)
}
