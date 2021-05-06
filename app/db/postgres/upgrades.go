package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/lib/pq"
)

const (
	upgradesQuery             = "upgrades.sql"
	upgradeVotingHistoryQuery = "upgradeVotingHistory.sql"
	upgradeQuery              = "upgrade.sql"
)

func (a *postgresAccessor) Upgrades(count uint64, continuationToken *string) ([]types.BlockSummary, *string, error) {
	res, nextContinuationToken, err := a.page(upgradesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.BlockSummary
		var height uint64
		for rows.Next() {
			block := types.BlockSummary{
				Coins: types.AllCoins{},
			}
			var timestamp int64
			var upgrade sql.NullInt64
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
			); err != nil {
				return nil, 0, err
			}
			block.Height = height
			block.Timestamp = timestampToTimeUTC(timestamp)
			if upgrade.Valid {
				v := uint32(upgrade.Int64)
				block.Upgrade = &v
			}
			res = append(res, block)
		}
		return res, height, nil
	}, count, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.BlockSummary), nextContinuationToken, nil
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
	res.StartActivationDate = timestampToTimeUTC(startActivationDate)
	res.EndActivationDate = timestampToTimeUTC(endActivationDate)
	return res, nil
}
