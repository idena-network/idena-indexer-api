package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/shopspring/decimal"
	"time"
)

const (
	addressQuery                         = "address.sql"
	addressPenaltiesCountQuery           = "addressPenaltiesCount.sql"
	addressPenaltiesQuery                = "addressPenalties.sql"
	addressStatesCountQuery              = "addressStatesCount.sql"
	addressStatesQuery                   = "addressStates.sql"
	addressTotalLatestMiningRewardQuery  = "addressTotalLatestMiningReward.sql"
	addressTotalLatestBurntCoinsQuery    = "addressTotalLatestBurntCoins.sql"
	addressBadAuthorsCountQuery          = "addressBadAuthorsCount.sql"
	addressBadAuthorsQuery               = "addressBadAuthors.sql"
	addressBalanceUpdatesCountQuery      = "addressBalanceUpdatesCount.sql"
	addressBalanceUpdatesQuery           = "addressBalanceUpdates.sql"
	addressBalanceUpdatesSummaryQuery    = "addressBalanceUpdatesSummary.sql"
	addressContractTxBalanceUpdatesQuery = "addressContractTxBalanceUpdates.sql"
	addressDelegateeTotalRewardsQuery    = "addressDelegateeTotalRewards.sql"
	addressMiningRewardSummariesQuery    = "addressMiningRewardSummaries.sql"
	addressTokensQuery                   = "addressTokens.sql"
	addressTokenQuery                    = "addressToken.sql"
	addressDelegationsQuery              = "addressDelegations.sql"

	txBalanceUpdateReason              = "Tx"
	committeeRewardBalanceUpdateReason = "CommitteeReward"
	contractBalanceUpdateReason        = "Contract"
	epochRewardBalanceUpdateReason     = "EpochReward"
)

func (a *postgresAccessor) Address(address string) (types.Address, error) {
	res := types.Address{}
	err := a.db.QueryRow(a.getQuery(addressQuery), address).Scan(
		&res.Address,
		&res.Balance,
		&res.Stake,
		&res.TxCount,
		&res.FlipsCount,
		&res.ReportedFlipsCount,
		&res.TokenCount,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.Address{}, err
	}
	return res, nil
}

func (a *postgresAccessor) AddressPenaltiesCount(address string) (uint64, error) {
	return a.count(addressPenaltiesCountQuery, address)
}

func (a *postgresAccessor) AddressPenalties(address string, count uint64, continuationToken *string) ([]types.Penalty, *string, error) {
	res, nextContinuationToken, err := a.page(addressPenaltiesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.Penalty
		var id uint64
		for rows.Next() {
			item := types.Penalty{}
			var timestamp int64
			if err := rows.Scan(
				&id,
				&item.Address,
				&item.Penalty,
				&item.PenaltySeconds,
				&item.BlockHeight,
				&item.BlockHash,
				&timestamp,
				&item.Epoch,
			); err != nil {
				return nil, 0, err
			}
			item.Timestamp = timestampToTimeUTC(timestamp)
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.Penalty), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressStatesCount(address string) (uint64, error) {
	return a.count(addressStatesCountQuery, address)
}

func (a *postgresAccessor) AddressStates(address string, count uint64, continuationToken *string) ([]types.AddressState, *string, error) {
	res, nextContinuationToken, err := a.page(addressStatesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.AddressState
		var id uint64
		for rows.Next() {
			item := types.AddressState{}
			var timestamp int64
			if err := rows.Scan(
				&id,
				&item.State,
				&item.Epoch,
				&item.BlockHeight,
				&item.BlockHash,
				&item.TxHash,
				&timestamp,
				&item.IsValidation,
			); err != nil {
				return nil, 0, err
			}
			item.Timestamp = timestampToTimeUTC(timestamp)
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.AddressState), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressTotalLatestMiningReward(afterTime time.Time, address string) (types.TotalMiningReward, error) {
	res := types.TotalMiningReward{}
	err := a.db.QueryRow(a.getQuery(addressTotalLatestMiningRewardQuery), afterTime.Unix(), address).
		Scan(&res.Balance, &res.Stake, &res.Proposer, &res.FinalCommittee)
	if err != nil {
		return types.TotalMiningReward{}, err
	}
	return res, nil
}

func (a *postgresAccessor) AddressTotalLatestBurntCoins(afterTime time.Time, address string) (types.AddressBurntCoins, error) {
	res := types.AddressBurntCoins{}
	err := a.db.QueryRow(a.getQuery(addressTotalLatestBurntCoinsQuery), afterTime.Unix(), address).
		Scan(&res.Amount)
	if err != nil {
		return types.AddressBurntCoins{}, err
	}
	return res, nil
}

func (a *postgresAccessor) AddressBadAuthorsCount(address string) (uint64, error) {
	return a.count(addressBadAuthorsCountQuery, address)
}

func (a *postgresAccessor) AddressBadAuthors(address string, count uint64, continuationToken *string) ([]types.BadAuthor, *string, error) {
	res, nextContinuationToken, err := a.page(addressBadAuthorsQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		return readBadAuthors(rows)
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.BadAuthor), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressBalanceUpdatesCount(address string) (uint64, error) {
	return a.count(addressBalanceUpdatesCountQuery, address)
}

type balanceUpdateOptionalData struct {
	txHash             string
	lastBlockHeight    uint64
	lastBlockHash      string
	lastBlockTimestamp int64
	rewardShare        decimal.Decimal
	blocksCount        uint32
	contractAddress    string
	epoch              uint64
}

func (a *postgresAccessor) AddressBalanceUpdates(address string, count uint64, continuationToken *string) ([]types.BalanceUpdate, *string, error) {
	res, nextContinuationToken, err := a.page2(addressBalanceUpdatesQuery, func(rows *sql.Rows) (interface{}, int64, error) {
		defer rows.Close()
		var res []types.BalanceUpdate
		var id int64
		for rows.Next() {
			item := types.BalanceUpdate{}
			var timestamp int64
			optionalData := &balanceUpdateOptionalData{}
			if err := rows.Scan(
				&id,
				&item.BalanceOld,
				&item.StakeOld,
				&item.PenaltyOld,
				&item.PenaltySecondsOld,
				&item.BalanceNew,
				&item.StakeNew,
				&item.PenaltyNew,
				&item.PenaltySecondsNew,
				&item.PenaltyPayment,
				&item.Reason,
				&item.BlockHeight,
				&item.BlockHash,
				&timestamp,
				&optionalData.txHash,
				&optionalData.lastBlockHeight,
				&optionalData.lastBlockHash,
				&optionalData.lastBlockTimestamp,
				&optionalData.rewardShare,
				&optionalData.blocksCount,
				&optionalData.epoch,
				&optionalData.contractAddress,
			); err != nil {
				return nil, 0, err
			}
			item.Timestamp = timestampToTimeUTC(timestamp)
			item.Data = readBalanceUpdateSpecificData(item.Reason, optionalData)
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.BalanceUpdate), nextContinuationToken, nil
}

func readBalanceUpdateSpecificData(reason string, optionalData *balanceUpdateOptionalData) interface{} {
	var res interface{}
	switch reason {
	case txBalanceUpdateReason:
		res = &types.TransactionBalanceUpdate{
			TxHash: optionalData.txHash,
		}
	case committeeRewardBalanceUpdateReason:
		res = &types.CommitteeRewardBalanceUpdate{
			LastBlockHeight:    optionalData.lastBlockHeight,
			LastBlockHash:      optionalData.lastBlockHash,
			LastBlockTimestamp: timestampToTimeUTC(optionalData.lastBlockTimestamp),
			RewardShare:        optionalData.rewardShare,
			BlocksCount:        optionalData.blocksCount,
		}
	case epochRewardBalanceUpdateReason:
		res = &types.EpochRewardBalanceUpdate{
			Epoch: optionalData.epoch,
		}
	case contractBalanceUpdateReason:
		res = &types.ContractBalanceUpdate{
			TransactionBalanceUpdate: types.TransactionBalanceUpdate{
				TxHash: optionalData.txHash,
			},
			ContractAddress: optionalData.contractAddress,
		}
	}
	return res
}

func (a *postgresAccessor) AddressContractTxBalanceUpdates(address string, contractAddress string, count uint64, continuationToken *string) ([]types.ContractTxBalanceUpdate, *string, error) {
	res, nextContinuationToken, err := a.page(addressContractTxBalanceUpdatesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.ContractTxBalanceUpdate
		var id uint64
		for rows.Next() {
			item := types.ContractTxBalanceUpdate{}
			var timestamp int64
			var balanceOld, balanceNew, gasCost NullDecimal
			var success sql.NullBool
			var gasUsed sql.NullInt64
			var method, errorMsg sql.NullString
			if err := rows.Scan(
				&id,
				&item.Hash,
				&item.Type,
				&timestamp,
				&item.From,
				&item.To,
				&item.Amount,
				&item.Tips,
				&item.MaxFee,
				&item.Fee,
				&item.Address,
				&item.ContractAddress,
				&item.ContractType,
				&balanceOld,
				&balanceNew,
				&success,
				&gasUsed,
				&gasCost,
				&method,
				&errorMsg,
			); err != nil {
				return nil, 0, err
			}
			item.Timestamp = timestampToTimeUTC(timestamp)
			if balanceOld.Valid && balanceNew.Valid {
				change := balanceNew.Decimal.Sub(balanceOld.Decimal)
				item.BalanceChange = &change
			}
			if success.Valid {
				item.TxReceipt = &types.TxReceipt{
					Success:  success.Bool,
					GasUsed:  uint64(gasUsed.Int64),
					GasCost:  gasCost.Decimal,
					Method:   method.String,
					ErrorMsg: errorMsg.String,
				}
			}
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, address, contractAddress)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.ContractTxBalanceUpdate), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressBalanceUpdatesSummary(address string) (*types.BalanceUpdatesSummary, error) {
	res := &types.BalanceUpdatesSummary{}
	err := a.db.QueryRow(a.getQuery(addressBalanceUpdatesSummaryQuery), address).Scan(
		&res.BalanceIn,
		&res.BalanceOut,
		&res.StakeIn,
		&res.StakeOut,
		&res.PenaltyIn,
		&res.PenaltyOut,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a *postgresAccessor) AddressDelegateeTotalRewards(address string, count uint64, continuationToken *string) ([]types.DelegateeTotalRewards, *string, error) {
	res, nextContinuationToken, err := a.page(addressDelegateeTotalRewardsQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.DelegateeTotalRewards
		var epoch uint64
		for rows.Next() {
			item := types.DelegateeTotalRewards{}
			var rewards validationRewards
			err := rows.Scan(
				&epoch,
				&rewards.validation,
				&rewards.flips,
				&rewards.extraFlips,
				&rewards.inv,
				&rewards.inv2,
				&rewards.inv3,
				&rewards.invitee,
				&rewards.invitee2,
				&rewards.invitee3,
				&rewards.savedInv,
				&rewards.savedInvWin,
				&rewards.reports,
				&rewards.candidate,
				&rewards.staking,
				&item.Delegators,
				&item.PenalizedDelegators,
			)
			if err != nil {
				return nil, 0, err
			}
			item.Rewards = toDelegationReward(rewards, a.replaceValidationReward)
			item.Epoch = epoch
			res = append(res, item)
		}
		return res, epoch, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.DelegateeTotalRewards), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressMiningRewardSummaries(address string, count uint64, continuationToken *string) ([]types.MiningRewardSummary, *string, error) {
	res, nextContinuationToken, err := a.page(addressMiningRewardSummariesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.MiningRewardSummary
		var epoch uint64
		for rows.Next() {
			item := types.MiningRewardSummary{}
			err := rows.Scan(
				&epoch,
				&item.Amount,
				&item.Penalty,
			)
			if err != nil {
				return nil, 0, err
			}
			item.Epoch = epoch
			res = append(res, item)
		}
		return res, epoch, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.MiningRewardSummary), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressTokens(address string, count uint64, continuationToken *string) ([]types.TokenBalance, *string, error) {
	res, nextContinuationToken, err := a.page(addressTokensQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.TokenBalance
		var id uint64
		for rows.Next() {
			item := types.TokenBalance{}
			err := rows.Scan(
				&id,
				&item.Token.ContractAddress,
				&item.Balance,
				&item.Token.Name,
				&item.Token.Symbol,
				&item.Token.Decimals,
			)
			if err != nil {
				return nil, 0, err
			}
			item.Address = address
			item.Balance = item.Balance.Div(decimal.New(1, int32(item.Token.Decimals)))
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.TokenBalance), nextContinuationToken, nil
}

func (a *postgresAccessor) AddressToken(address, tokenAddress string) (types.TokenBalance, error) {
	res := types.TokenBalance{}
	err := a.db.QueryRow(a.getQuery(addressTokenQuery), address, tokenAddress).Scan(
		&res.Balance,
		&res.Token.Name,
		&res.Token.Symbol,
		&res.Token.Decimals,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.TokenBalance{}, err
	}
	res.Address = address
	res.Token.ContractAddress = tokenAddress
	res.Balance = res.Balance.Div(decimal.New(1, int32(res.Token.Decimals)))
	return res, nil
}

func (a *postgresAccessor) AddressDelegations(address string, count uint64, continuationToken *string) ([]types.Delegation, *string, error) {
	res, nextContinuationToken, err := a.page(addressDelegationsQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.Delegation
		var id uint64
		for rows.Next() {
			item := types.Delegation{}
			var delegationTxTimestamp int64
			var delegationBlockHeight, delegationBlockEpoch, delegationBlockTimestamp, undelegationBlockHeight, undelegationBlockEpoch, undelegationBlockTimestamp, undelegationTxTimestamp sql.NullInt64
			var delegationBlockHash, undelegationBlockHash, undelegationTxHash, undelegationTxType, undelegationReason sql.NullString
			err := rows.Scan(
				&id,
				&item.DelegateeAddress,
				&item.DelegationTx.Hash,
				&delegationTxTimestamp,
				&delegationBlockHeight,
				&delegationBlockHash,
				&delegationBlockEpoch,
				&delegationBlockTimestamp,
				&undelegationTxHash,
				&undelegationTxType,
				&undelegationTxTimestamp,
				&undelegationBlockHeight,
				&undelegationBlockHash,
				&undelegationBlockEpoch,
				&undelegationBlockTimestamp,
				&undelegationReason,
			)
			if err != nil {
				return nil, 0, err
			}
			item.DelegationTx.Timestamp = timestampToTimeUTCp(delegationTxTimestamp)
			if delegationBlockHeight.Valid {
				item.DelegationBlock = &types.BlockSummary{}
				item.DelegationBlock.Height = uint64(delegationBlockHeight.Int64)
				item.DelegationBlock.Hash = delegationBlockHash.String
				item.DelegationBlock.Epoch = uint64(delegationBlockEpoch.Int64)
				item.DelegationBlock.Timestamp = timestampToTimeUTC(delegationBlockTimestamp.Int64)
			}
			if undelegationTxHash.Valid {
				item.UndelegationTx = &types.TransactionSummary{}
				item.UndelegationTx.Hash = undelegationTxHash.String
				item.UndelegationTx.Type = undelegationTxType.String
				item.UndelegationTx.Timestamp = timestampToTimeUTCp(undelegationTxTimestamp.Int64)
			}
			if undelegationBlockHeight.Valid {
				item.UndelegationBlock = &types.BlockSummary{}
				item.UndelegationBlock.Height = uint64(undelegationBlockHeight.Int64)
				item.UndelegationBlock.Hash = undelegationBlockHash.String
				item.UndelegationBlock.Epoch = uint64(undelegationBlockEpoch.Int64)
				item.UndelegationBlock.Timestamp = timestampToTimeUTC(undelegationBlockTimestamp.Int64)
			}
			if undelegationReason.Valid {
				item.UndelegationReason = undelegationReason.String
			}
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, address)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.Delegation), nextContinuationToken, nil
}
