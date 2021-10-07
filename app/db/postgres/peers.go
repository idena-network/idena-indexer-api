package postgres

import "github.com/idena-network/idena-indexer-api/app/types"

const peersHistoryQuery = "peersHistory.sql"

func (a *postgresAccessor) PeersHistory(count uint64) ([]types.PeersHistoryItem, error) {
	rows, err := a.db.Query(a.getQuery(peersHistoryQuery), count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.PeersHistoryItem
	for rows.Next() {
		item := types.PeersHistoryItem{}
		var timestamp int64
		err := rows.Scan(
			&timestamp,
			&item.Count,
		)
		if err != nil {
			return nil, err
		}
		item.Timestamp = timestampToTimeUTC(timestamp)
		res = append(res, item)
	}
	return res, nil
}
