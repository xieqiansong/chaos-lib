// Package dbmonitor 只读自省当前数据库：库级总览、表清单（行数 / 大小 / 索引数）、单表列与索引详情。
// 兼容 SQLite / PostgreSQL 双后端。
//
// 分层（见 chaos-lib/AGENTS.md「分层契约」）：
//   - model.go：领域错误
//   - repo_sqlite.go / repo_postgres.go：按方言拆分的数据访问（repository）
//   - service.go：分派、过滤、排序与用例
//   - dto.go：响应契约
//   - handler.go：参数解析 + 状态码映射 + 路由注册
package dbmonitor

import "errors"

// errTableNotFound 方言实现内部使用的「表不存在」哨兵。
var errTableNotFound = errors.New("table not found")

// 领域错误哨兵：调用方据此映射 HTTP 状态码。
var (
	ErrTableNotFound     = errors.New("dbmonitor: 表不存在")
	ErrInvalidTableName  = errors.New("dbmonitor: 非法的表名")
)
