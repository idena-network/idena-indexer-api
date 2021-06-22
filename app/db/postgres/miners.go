package postgres

import "github.com/idena-network/idena-indexer-api/app/types"

const minersHistoryQuery = "minersHistory.sql"

func (a *postgresAccessor) MinersHistory() ([]types.MinersHistoryItem, error) {
	rows, err := a.db.Query(a.getQuery(minersHistoryQuery))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.MinersHistoryItem
	for rows.Next() {
		item := types.MinersHistoryItem{}
		var timestamp int64
		err := rows.Scan(
			&timestamp,
			&item.OnlineMiners,
			&item.OnlineValidators,
		)
		if err != nil {
			return nil, err
		}
		item.Timestamp = timestampToTimeUTC(timestamp)
		res = append(res, item)
	}
	return res, nil
}
