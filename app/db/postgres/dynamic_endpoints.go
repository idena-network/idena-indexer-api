package postgres

import (
	"database/sql"
	"fmt"
	"github.com/idena-network/idena-indexer-api/app/types"
)

func (a *postgresAccessor) DynamicEndpoints() ([]types.DynamicEndpoint, error) {
	rows, err := a.db.Query(fmt.Sprintf("SELECT name, endpoint_method, \"limit\" FROM %v", a.dynamicEndpointsTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.DynamicEndpoint
	for rows.Next() {
		item := types.DynamicEndpoint{}
		var limit sql.NullInt32
		err = rows.Scan(
			&item.DataSource,
			&item.Method,
			&limit,
		)
		if err != nil {
			return nil, err
		}
		if limit.Valid {
			v := int(limit.Int32)
			item.Limit = &v
		}
		if len(item.Method) == 0 {
			item.Method = item.DataSource
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) DynamicEndpointData(name string, limit *int) (*types.DynamicEndpointResult, error) {
	var rows *sql.Rows
	var err error
	if limit != nil {
		rows, err = a.db.Query(fmt.Sprintf("SELECT t.*, t2.last_refresh_time FROM %v AS t LEFT JOIN (SELECT last_refresh_time FROM %v WHERE name = '%v') t2 ON true LIMIT %d", name, a.dynamicEndpointStatesTable, name, *limit))
	} else {
		rows, err = a.db.Query(fmt.Sprintf("SELECT t.*, t2.last_refresh_time FROM %v AS t LEFT JOIN (SELECT last_refresh_time FROM %v WHERE name = '%v') t2 ON true", name, a.dynamicEndpointStatesTable, name))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &types.DynamicEndpointResult{}
	isFirst := true
	for rows.Next() {
		item := make(map[string]interface{})
		if err := scanMap(rows, item); err != nil {
			return nil, err
		}
		if isFirst {
			if refreshTime, ok := item["last_refresh_time"].(int64); ok {
				v := timestampToTimeUTC(refreshTime)
				res.Date = &v

			}
			isFirst = false
		}
		delete(item, "last_refresh_time")
		res.Data = append(res.Data, item)
	}
	return res, nil
}

func scanMap(r *sql.Rows, dest map[string]interface{}) error {
	const NUMERIC = "NUMERIC"
	columns, err := r.Columns()
	if err != nil {
		return err
	}
	columnTypes, err := r.ColumnTypes()
	if err != nil {
		return err
	}
	values := make([]interface{}, len(columns))
	for i := range values {
		if columnTypes[i].DatabaseTypeName() == NUMERIC {
			values[i] = new(string)
		} else {
			values[i] = new(interface{})
		}
	}
	err = r.Scan(values...)
	if err != nil {
		return err
	}
	for i, column := range columns {
		if columnTypes[i].DatabaseTypeName() == NUMERIC {
			dest[column] = *(values[i].(*string))
		} else {
			dest[column] = *(values[i].(*interface{}))
		}
	}
	return r.Err()
}
