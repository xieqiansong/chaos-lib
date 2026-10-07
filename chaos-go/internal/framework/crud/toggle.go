package crud

import (
	"chaos-go/internal/framework/httpx"
)

// ToggleOpts 启停（status 动作）的配置。
type ToggleOpts struct {
	// Setter 业务实现：按 ID 与目标状态执行切换（含副作用），返回响应载荷。
	// 编排属于用例层，业务包应把它实现在 service.go 里，这里只做一行转调。
	Setter func(id int, status bool) (any, error)
	// ErrRules 领域错误 → HTTP 状态码映射表；未命中回落 500。
	ErrRules []httpx.ErrRule
}
