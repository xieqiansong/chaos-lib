// Package systemjob 提供由原系统内置周期任务改造而来的内部动作接口，
// 供定时任务模块通过 HTTP 触发。本包不持有自己的模型，仅做跨模块动作编排。
package systemjob

import (
	"chaos-go/internal/portfwd"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"
	"chaos-go/internal/stunpf"
	"chaos-go/internal/stunsync"
	"chaos-go/internal/taskplan"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("system-jobs", Register)
}

// Register 挂载内部动作接口，均触发有副作用的下游动作并以标准响应返回。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/system-jobs")
	g.POST("/sweep", func(c *gin.Context) {
		taskplan.SweepScheduledTaskPlans()
		renv.Success(c, nil)
	})
	g.POST("/portForwardSelfHeal", func(c *gin.Context) {
		portfwd.SelfHealForwards()
		renv.Success(c, nil)
	})
	g.POST("/stunRuleSync", func(c *gin.Context) {
		stunsync.RunSync()
		renv.Success(c, nil)
	})
	g.POST("/stunPortForwardSync", func(c *gin.Context) {
		stunpf.RunSync()
		renv.Success(c, nil)
	})
}
