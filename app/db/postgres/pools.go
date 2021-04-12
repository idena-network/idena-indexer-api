package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/pkg/errors"
	"strconv"
	"strings"
)

const (
	poolsCountQuery          = "poolsCount.sql"
	poolsQuery               = "pools.sql"
	poolQuery                = "pool.sql"
	poolDelegatorsCountQuery = "poolDelegatorsCount.sql"
	poolDelegatorsQuery      = "poolDelegators.sql"
)

func (a *postgresAccessor) PoolsCount() (uint64, error) {
	return a.count(poolsCountQuery)
}

func (a *postgresAccessor) Pools(count uint64, continuationToken *string) ([]*types.Pool, *string, error) {
	parseToken := func(continuationToken *string) (addressId, size *uint64, err error) {
		if continuationToken == nil {
			return
		}
		strs := strings.Split(*continuationToken, "-")
		if len(strs) != 2 {
			err = errors.New("invalid continuation token")
			return
		}
		if addressId, err = parseUintContinuationToken(&strs[0]); err != nil {
			return
		}
		if size, err = parseUintContinuationToken(&strs[1]); err != nil {
			return
		}
		return
	}
	addressId, size, err := parseToken(continuationToken)
	if err != nil {
		return nil, nil, err
	}
	rows, err := a.db.Query(a.getQuery(poolsQuery), count+1, addressId, size)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var res []*types.Pool
	for rows.Next() {
		item := &types.Pool{}
		err = rows.Scan(
			&addressId,
			&item.Address,
			&item.Size,
		)
		if err != nil {
			return nil, nil, err
		}
		res = append(res, item)
	}
	var nextContinuationToken *string
	if len(res) > 0 && len(res) == int(count)+1 {
		t := strconv.FormatUint(*addressId, 10) + "-" + strconv.FormatUint(res[len(res)-1].Size, 10)
		nextContinuationToken = &t
		res = res[:len(res)-1]
	}
	return res, nextContinuationToken, nil
}

func (a *postgresAccessor) Pool(address string) (*types.Pool, error) {
	res := &types.Pool{}
	err := a.db.QueryRow(a.getQuery(poolQuery), address).Scan(
		&res.Address,
		&res.Size,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a *postgresAccessor) PoolDelegatorsCount(address string) (uint64, error) {
	return a.count(poolDelegatorsCountQuery, address)
}

func (a *postgresAccessor) PoolDelegators(address string, count uint64, continuationToken *string) ([]*types.Delegator, *string, error) {
	parseToken := func(continuationToken *string) (addressId, birthEpoch *uint64, err error) {
		if continuationToken == nil {
			return
		}
		strs := strings.Split(*continuationToken, "-")
		if len(strs) != 2 {
			err = errors.New("invalid continuation token")
			return
		}
		if addressId, err = parseUintContinuationToken(&strs[0]); err != nil {
			return
		}
		if birthEpoch, err = parseUintContinuationToken(&strs[1]); err != nil {
			return
		}
		return
	}
	addressId, birthEpoch, err := parseToken(continuationToken)
	if err != nil {
		return nil, nil, err
	}
	rows, err := a.db.Query(a.getQuery(poolDelegatorsQuery), count+1, addressId, birthEpoch, address)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var res []*types.Delegator
	for rows.Next() {
		item := &types.Delegator{}
		err = rows.Scan(
			&addressId,
			&birthEpoch,
			&item.Address,
			&item.State,
			&item.Age,
		)
		if err != nil {
			return nil, nil, err
		}
		res = append(res, item)
	}
	var nextContinuationToken *string
	if len(res) > 0 && len(res) == int(count)+1 {
		t := strconv.FormatUint(*addressId, 10) + "-" + strconv.FormatUint(*birthEpoch, 10)
		nextContinuationToken = &t
		res = res[:len(res)-1]
	}
	return res, nextContinuationToken, nil
}
