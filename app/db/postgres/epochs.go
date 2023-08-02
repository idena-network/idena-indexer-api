package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
)

const (
	epochsCountQuery = "epochsCount.sql"
	epochsQuery      = "epochs.sql"
)

func (a *postgresAccessor) EpochsCount() (uint64, error) {
	return a.count(epochsCountQuery)
}

func (a *postgresAccessor) Epochs(count uint64, continuationToken *string) ([]types.EpochSummary, *string, error) {
	res, nextContinuationToken, err := a.page(epochsQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.EpochSummary
		var epoch uint64
		for rows.Next() {
			item := types.EpochSummary{
				Coins:   types.AllCoins{},
				Rewards: types.RewardsSummary{},
			}
			var validationTime int64
			var discriminationStakeThreshold NullDecimal
			if err := rows.Scan(
				&epoch,
				&validationTime,
				&item.ValidatedCount,
				&item.BlockCount,
				&item.EmptyBlockCount,
				&item.TxCount,
				&item.InviteCount,
				&item.FlipCount,
				&item.Coins.Burnt,
				&item.Coins.Minted,
				&item.Coins.TotalBalance,
				&item.Coins.TotalStake,
				&item.Rewards.Total,
				&item.Rewards.Validation,
				&item.Rewards.Flips,
				&item.Rewards.Invitations,
				&item.Rewards.Reports,
				&item.Rewards.Candidate,
				&item.Rewards.Staking,
				&item.Rewards.FoundationPayouts,
				&item.Rewards.ZeroWalletFund,
				&item.MinScoreForInvite,
				&item.CandidateCount,
				&discriminationStakeThreshold,
			); err != nil {
				return nil, 0, err
			}
			item.ValidationTime = timestampToTimeUTC(validationTime)
			item.Epoch = epoch
			if a.replaceValidationReward {
				if item.Rewards.Candidate.Sign() > 0 || item.Rewards.Staking.Sign() > 0 {
					item.Rewards.Validation = item.Rewards.Candidate.Add(item.Rewards.Staking)
				}
			}
			if discriminationStakeThreshold.Valid {
				item.DiscriminationStakeThreshold = &discriminationStakeThreshold.Decimal
			}
			res = append(res, item)
		}
		return res, epoch, nil
	}, count, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.EpochSummary), nextContinuationToken, nil
}
