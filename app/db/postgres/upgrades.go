package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

const (
	upgradesQuery             = "upgrades.sql"
	upgradeVotingHistoryQuery = "upgradeVotingHistory.sql"
	upgradeQuery              = "upgrade.sql"
	upgradeVotingsQuery       = "upgradeVotings.sql"
)

func (a *postgresAccessor) Upgrades(count uint64, continuationToken *string) ([]types.ActivatedUpgrade, *string, error) {
	res, nextContinuationToken, err := a.page(upgradesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.ActivatedUpgrade
		var height uint64
		for rows.Next() {
			block := types.ActivatedUpgrade{
				BlockSummary: types.BlockSummary{Coins: types.AllCoins{}},
			}
			var timestamp int64
			var upgrade sql.NullInt64
			var offlineAddress sql.NullString
			if err := rows.Scan(&height,
				&block.Hash,
				&timestamp,
				&block.TxCount,
				&block.Proposer,
				&block.ProposerVrfScore,
				&block.IsEmpty,
				&block.BodySize,
				&block.FullSize,
				&block.VrfProposerThreshold,
				&block.FeeRate,
				&block.Coins.Burnt,
				&block.Coins.Minted,
				&block.Coins.TotalBalance,
				&block.Coins.TotalStake,
				pq.Array(&block.Flags),
				&upgrade,
				&offlineAddress,
			); err != nil {
				return nil, 0, err
			}
			block.Height = height
			block.Timestamp = timestampToTimeUTC(timestamp)
			if upgrade.Valid {
				v := uint32(upgrade.Int64)
				block.Upgrade = &v
			}
			if offlineAddress.Valid {
				block.OfflineAddress = &offlineAddress.String
			}
			if block.Height < a.embeddedContractForkHeight {
				block.FeeRate = block.FeeRate.Div(decimal.NewFromInt(10))
			}
			res = append(res, block)
		}
		return res, height, nil
	}, count, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.ActivatedUpgrade), nextContinuationToken, nil
}

func (a *postgresAccessor) UpgradeVotingHistory(upgrade uint64) ([]*types.UpgradeVotingHistoryItem, error) {
	rows, err := a.db.Query(a.getQuery(upgradeVotingHistoryQuery), upgrade)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*types.UpgradeVotingHistoryItem
	for rows.Next() {
		item := &types.UpgradeVotingHistoryItem{}
		var timestamp int64
		var blockHeight uint64
		err := rows.Scan(
			&blockHeight,
			&timestamp,
			&item.Votes,
		)
		if err != nil {
			return nil, err
		}
		item.Timestamp = timestampToTimeUTC(timestamp)
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) Upgrade(upgrade uint64) (*types.Upgrade, error) {
	res := &types.Upgrade{}
	var startActivationDate, endActivationDate int64
	err := a.db.QueryRow(a.getQuery(upgradeQuery), upgrade).Scan(
		&startActivationDate,
		&endActivationDate,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return nil, err
	}
	{
		v := timestampToTimeUTC(startActivationDate)
		res.StartActivationDate = &v
	}
	{
		v := timestampToTimeUTC(endActivationDate)
		res.EndActivationDate = &v
	}
	return res, nil
}

func (a *postgresAccessor) UpgradeVotings(count uint64, continuationToken *string) ([]types.Upgrade, *string, error) {
	res, nextContinuationToken, err := a.page(upgradeVotingsQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.Upgrade
		var upgrade uint64
		for rows.Next() {
			item := types.Upgrade{}
			var startActivationDate, endActivationDate int64
			if err := rows.Scan(
				&upgrade,
				&startActivationDate,
				&endActivationDate,
			); err != nil {
				return nil, 0, err
			}
			item.Upgrade = uint32(upgrade)
			{
				v := timestampToTimeUTC(startActivationDate)
				item.StartActivationDate = &v
			}
			{
				v := timestampToTimeUTC(endActivationDate)
				item.EndActivationDate = &v
			}
			res = append(res, item)
		}
		return res, upgrade, nil
	}, count, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.Upgrade), nextContinuationToken, nil
}
