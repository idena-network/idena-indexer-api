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

func (a *postgresAccessor) DynamicEndpointData(name string, limit *int) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error
	if limit != nil {
		rows, err = a.db.Query(fmt.Sprintf("SELECT * FROM %v LIMIT %d", name, *limit))
	} else {
		rows, err = a.db.Query(fmt.Sprintf("SELECT * FROM %v", name))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []map[string]interface{}
	for rows.Next() {
		item := make(map[string]interface{})
		if err := scanMap(rows, item); err != nil {
			return nil, err
		}
		res = append(res, item)
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
