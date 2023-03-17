package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
)

const (
	tokenQuery = "token.sql"
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
