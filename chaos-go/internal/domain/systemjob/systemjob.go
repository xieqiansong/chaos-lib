// Package systemjob 提供由原系统内置周期任务改造而来的内部动作接口，
// 供定时任务模块通过 HTTP 触发。本包不持有自己的模型，仅做跨模块动作编排。
package systemjob

import (
	"chaos-go/internal/domain/portfwd"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"
	"chaos-go/internal/platform/stunpf"
	"chaos-go/internal/platform/stunsync"
	"chaos-go/internal/domain/taskplan"

	"chaos-go/internal/framework/web"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.RegisterV1("system-jobs", RegisterV1)
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 动作均无需参数，直接复用内部编排逻辑。
func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/system-jobs")
	g.POST("/sweep", func(c *web.Context) {
		taskplan.SweepScheduledTaskPlans()
		renv.Success(c, nil)
	})
	g.POST("/portForwardSelfHeal", func(c *web.Context) {
		portfwd.SelfHealForwards()
		renv.Success(c, nil)
	})
	g.POST("/stunRuleSync", func(c *web.Context) {
		stunsync.RunSync()
		renv.Success(c, nil)
	})
	g.POST("/stunPortForwardSync", func(c *web.Context) {
		stunpf.RunSync()
		renv.Success(c, nil)
	})
}
