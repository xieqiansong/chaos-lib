package tools

import (
	"chaos-go/config"
	"database/sql"

	_ "github.com/lib/pq"
)

// QueryRows 执行任意 SELECT，返回 []map[string]any（列名 -> 值）。
// 每行就是一个不定结构的 map，无需预先定义结构体。
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
		// 每个字段一个 *any，驱动把值填进去
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
			// lib/pq 把文本列扫成 []byte，转成 string 更方便使用
			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v // int64 / float64 / bool / time.Time / nil
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetDb 从 .env 加载数据库配置并打开连接，返回 *sql.DB。
// 配置项（含密码）一律来自 .env，源码中不硬编码任何密钥，满足脱敏要求。
// 当前使用 postgres 驱动（database/sql + lib/pq）。
func GetDb() *sql.DB {
	cfg := config.GetConfig().Database
	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		panic(err)
	}
	return db
}
