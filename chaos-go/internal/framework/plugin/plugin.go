// Package plugin 提供 chaos-go 的插件注册与依赖编排框架。
//
// 设计目标：在不破坏既有 routehub 路由挂载的前提下，为各业务模块补上
// 「元数据 + 依赖声明 + 生命周期编排」。每个模块在自己的包内调用 Register
// 登记自身（ID、依赖、模型、路由挂载、启动/停止钩子），装配层（internal/app）
// 在启动早期调用 Build 做依赖校验（缺失依赖 / 循环依赖 fail-fast），再按依赖
// 拓扑序依次 Migrate / Start。
//
// 约定：
//   - framework（config/db/crud/routehub/scheduler 等）视为隐式根，无需声明依赖；
//   - 业务插件之间若需保证先后顺序，才在 Dependencies 中显式声明；
//   - 所有钩子字段均可为 nil（按需实现），未实现即跳过；
//   - 循环依赖中，标记为 Soft（接口型）的边不参与硬环判定（如 envvar↔quickedit）。
package plugin

import (
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

// Meta 是插件的静态描述，依赖校验与拓扑排序的唯一依据。
type Meta struct {
	ID           string   // 稳定标识，如 "taskplan"
	Name         string   // 展示名
	Version      string   // semver
	Dependencies []string // 硬依赖的其它插件 ID（仅业务插件间；framework 视为隐式根）
	// SoftDeps 是「接口型」依赖：运行时通过插件上下文查表获取服务，不形成硬环。
	// 例如 quickedit 依赖 envvar 提供的虚拟文件服务，但不 import envvar 包本体。
	SoftDeps []string
	// Disabled 为 true 时插件不启用；缺省（零值）即启用，符合「默认全启用」契约。
	Disabled bool
}

// IsEnabled 插件是否启用（未显式 Disabled 即视为启用）。
func (m Meta) IsEnabled() bool { return !m.Disabled }

// Registration 是一次插件登记。除 Meta 外所有字段均可为 nil（按需实现）。
type Registration struct {
	Meta           Meta
	Models         []any                             // 交给 GORM AutoMigrate 的模型
	Configure      func(cfg *config.AppConfig) error // 配置注入（如读取专属配置段）
	RegisterRoutes func(rg *web.RouterGroup)         // 内部仍调 routehub.RegisterV1
	Tasks          []scheduler.TaskSpec              // 插件自有的内置定时任务（代码定义，不落库）
	Start          func() error                      // 后台任务 / 恢复期等
	Stop           func() error
}
