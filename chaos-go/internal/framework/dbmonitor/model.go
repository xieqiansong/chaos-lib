package dbmonitor

import "errors"

var errTableNotFound = errors.New("table not found")

var (
	ErrTableNotFound    = errors.New("dbmonitor: 表不存在")
	ErrInvalidTableName = errors.New("dbmonitor: 非法的表名")
)
