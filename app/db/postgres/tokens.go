package postgres

import (
	"database/sql"
	"fmt"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"strings"
)

const (
	tokenQuery        = "token.sql"
	tokenHoldersQuery = "tokenHolders.sql"
)

func (a *postgresAccessor) Token(address string) (types.Token, error) {
	var res types.Token
	err := a.db.QueryRow(a.getQuery(tokenQuery), address).Scan(
		&res.ContractAddress,
		&res.Name,
		&res.Symbol,
		&res.Decimals,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.Token{}, err
	}
	return res, nil
}

func (a *postgresAccessor) TokenHolders(address string, count uint64, continuationToken *string) ([]types.TokenBalance, *string, error) {
	parseToken := func(continuationToken *string) (address, balance *string, err error) {
		if continuationToken == nil {
			return
		}
		strs := strings.Split(*continuationToken, "-")
		if len(strs) != 2 {
			err = errors.New("invalid continuation token")
			return
		}
		address = &strs[0]
		balance = &strs[1]
		return
	}
	continuationTokenAddress, continuationTokenBalance, err := parseToken(continuationToken)
	if err != nil {
		return nil, nil, err
	}
	rows, err := a.db.Query(a.getQuery(tokenHoldersQuery), address, count+1, continuationTokenAddress, continuationTokenBalance)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var res []types.TokenBalance
	for rows.Next() {
		var item types.TokenBalance
		err = rows.Scan(
			&item.Address,
			&item.Balance,
			&item.Token.Name,
			&item.Token.Symbol,
			&item.Token.Decimals,
		)
		if err != nil {
			return nil, nil, err
		}
		{
			continuationTokenAddress = &item.Address
			continuationTokenBalanceV := item.Balance.String()
			continuationTokenBalance = &continuationTokenBalanceV
		}
		item.Token.ContractAddress = address
		item.Balance = item.Balance.Div(decimal.New(1, int32(item.Token.Decimals)))
		res = append(res, item)
	}
	var nextContinuationToken *string
	if len(res) > 0 && len(res) == int(count)+1 {
		t := fmt.Sprintf("%v-%v", *continuationTokenAddress, *continuationTokenBalance)
		nextContinuationToken = &t
		res = res[:len(res)-1]
	}
	return res, nextContinuationToken, nil
}
