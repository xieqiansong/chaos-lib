package systemjob

import (
	"chaos-go/internal/domain/portfwd"
	"chaos-go/internal/domain/taskplan"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/platform/stunpf"
	"chaos-go/internal/platform/stunsync"

	"chaos-go/internal/framework/web"
)

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
