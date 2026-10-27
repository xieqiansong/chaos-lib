package tools

import (
	"chaos-go/internal/framework/config"
	"database/sql"

	_ "github.com/lib/pq"
)

func QueryRows(query string, args ...any) ([]map[string]any, error) {
	db := GetDb()
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer CloseQuietly(rows)

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0)
	for rows.Next() {

		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}

		row := make(map[string]any, len(cols))
		for i, col := range cols {
			v := values[i]

			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func GetDb() *sql.DB {
	cfg := config.GetConfig().Database
	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		panic(err)
	}
	return db
}
