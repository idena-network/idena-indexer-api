package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

const (
	blockQueryByHeight         = "blockByHeight.sql"
	blockQueryByHash           = "blockByHash.sql"
	blockTxsCountByHeightQuery = "blockTxsCountByHeight.sql"
	blockTxsCountByHashQuery   = "blockTxsCountByHash.sql"
	blockTxsByHeightQuery      = "blockTxsByHeight.sql"
	blockTxsByHashQuery        = "blockTxsByHash.sql"
	blockCoinsByHeightQuery    = "blockCoinsByHeight.sql"
	blockCoinsByHashQuery      = "blockCoinsByHash.sql"
	lastBlockQuery             = "lastBlock.sql"
)

func (a *postgresAccessor) BlockByHeight(height uint64) (types.BlockDetail, error) {
	return a.block(blockQueryByHeight, height)
}

func (a *postgresAccessor) BlockByHash(hash string) (types.BlockDetail, error) {
	return a.block(blockQueryByHash, hash)
}

func (a *postgresAccessor) LastBlock() (types.BlockDetail, error) {
	return a.block(lastBlockQuery)
}

func (a *postgresAccessor) block(query string, args ...interface{}) (types.BlockDetail, error) {
	res := types.BlockDetail{}
	var timestamp int64
	var upgrade sql.NullInt64
	var offlineAddress sql.NullString
	var blockFeeRate decimal.Decimal
	err := a.db.QueryRow(a.getQuery(query), args...).Scan(
		&res.Epoch,
		&res.Height,
		&res.Hash,
		&timestamp,
		&res.TxCount,
		&res.ValidatorsCount,
		&res.Proposer,
		&res.ProposerVrfScore,
		&res.IsEmpty,
		&res.BodySize,
		&res.FullSize,
		&res.VrfProposerThreshold,
		&blockFeeRate,
		pq.Array(&res.Flags),
		&upgrade,
		&offlineAddress,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.BlockDetail{}, err
	}
	res.Timestamp = timestampToTimeUTC(timestamp)
	if upgrade.Valid {
		v := uint32(upgrade.Int64)
		res.Upgrade = &v
	}
	if offlineAddress.Valid {
		res.OfflineAddress = &offlineAddress.String
	}
	res.FeeRate, res.FeeRatePerByte = feeRate(blockFeeRate, res.Height, a.embeddedContractForkHeight)
	return res, nil
}

func (a *postgresAccessor) BlockTxsCountByHeight(height uint64) (uint64, error) {
	return a.count(blockTxsCountByHeightQuery, height)
}

func (a *postgresAccessor) BlockTxsCountByHash(hash string) (uint64, error) {
	return a.count(blockTxsCountByHashQuery, hash)
}

func (a *postgresAccessor) BlockTxsByHeight(height uint64, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	res, nextContinuationToken, err := a.page(blockTxsByHeightQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		return readTxs(rows)
	}, count, continuationToken, height)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.TransactionSummary), nextContinuationToken, nil
}

func (a *postgresAccessor) BlockTxsByHash(hash string, count uint64, continuationToken *string) ([]types.TransactionSummary, *string, error) {
	res, nextContinuationToken, err := a.page(blockTxsByHashQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		return readTxs(rows)
	}, count, continuationToken, hash)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.TransactionSummary), nextContinuationToken, nil
}

func (a *postgresAccessor) BlockCoinsByHeight(height uint64) (types.AllCoins, error) {
	return a.coins(blockCoinsByHeightQuery, height)
}

func (a *postgresAccessor) BlockCoinsByHash(hash string) (types.AllCoins, error) {
	return a.coins(blockCoinsByHashQuery, hash)
}
