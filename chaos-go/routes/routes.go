package routes

import (
	"chaos-go/internal/cronjob"
	"chaos-go/internal/dbmonitor"
	"chaos-go/internal/envvar"
	"chaos-go/internal/filelink"
	mqttsync "chaos-go/internal/mqttsync"
	notifysvc "chaos-go/internal/notify"
	"chaos-go/internal/portfwd"
	"chaos-go/internal/project"
	"chaos-go/internal/proxy"
	"chaos-go/internal/quickedit"
	"chaos-go/internal/standarddatas"
	"chaos-go/internal/stunpf"
	stunsync "chaos-go/internal/stunsync"
	"chaos-go/internal/taskplan"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetupRouter(webFS fs.FS) *gin.Engine {
	r := gin.Default()

	r.Use(gzip.Gzip(gzip.DefaultCompression))

	api := r.Group("/api")
	{
		api.GET("/browserHistories", proxy.GetBrowserHistories)
		api.POST("/browserHistories", proxy.SaveBrowserHistory)
		api.POST("/browserHistoryVisits", proxy.SaveBrowserHistoryVisits)

		// 常用书签：独立接口（书签 ∪ 历史访问次数，按访问频率排序，分页）
		api.GET("/frequentBookmarks", proxy.GetFrequentBookmarks)
		api.POST("/bookmarks", proxy.SaveBookmarks)

		api.GET("/sdks", proxy.GetSdkVersions)
		api.GET("/sdks/:type", proxy.GetSdkVersion)
		api.PATCH("/sdks/:type/switch", proxy.UpdateSdkVersion)

		api.GET("/sdks/defs", proxy.ListSdkSources)
		api.POST("/sdks/defs", proxy.CreateSdkSource)
		api.PATCH("/sdks/defs/:name", proxy.UpdateSdkSource)
		api.DELETE("/sdks/defs/:name", proxy.DeleteSdkSource)

		api.GET("/fileLinks", filelink.GetFileLinks)
		api.POST("/fileLinks", filelink.CreateFileLink)
		api.DELETE("/fileLinks/:id", filelink.DeleteFileLink)
		api.PATCH("/fileLinks/:id", filelink.UpdateFileLink)
		api.PATCH("/fileLinks/:id/status", filelink.UpdateFileLinkStatus)

		// 标准参考表：资源接口自包含，本行仅做编排调用（路由实现在 internal/standarddatas）
		standarddatas.Register(api)

		quickEdits := api.Group("/quickEdits")
		{
			quickEdits.GET("/", quickedit.ListQuickEdits)
			quickEdits.POST("/", quickedit.CreateQuickEdit)
			quickEdits.DELETE("/:id", quickedit.DeleteQuickEdit)
			quickEdits.GET("/:id/content", quickedit.GetQuickEditContent)
			quickEdits.PUT("/:id/content", quickedit.UpdateQuickEditContent)
			quickEdits.GET("/:id/snapshots", quickedit.ListQuickEditSnapshots)
			quickEdits.GET("/:id/snapshots/:snapshotId", quickedit.GetQuickEditSnapshot)
			quickEdits.POST("/:id/restore", quickedit.RestoreQuickEdit)
		}

		envVars := api.Group("/envVariables")
		{
			envVars.GET("/", envvar.GetEnvVariables)
			envVars.PATCH("/", envvar.PatchEnvVariables)
			envVars.PUT("/", envvar.PutEnvVariables)
			envVars.POST("/sync", envvar.SyncEnvVariables)
			envVars.GET("/snapshots/:snapshotId", envvar.GetEnvSnapshotDetail)
		}

		// MQTT 多节点消息同步（公共 broker）
		mqttSync := api.Group("/mqttSync")
		{
			mqttSync.GET("/messages", mqttsync.ListMessages)
			mqttSync.POST("/messages", mqttsync.SendMessage)
			mqttSync.DELETE("/messages", mqttsync.DeleteMessages)
			mqttSync.GET("/status", mqttsync.Status)
		}

		taskPlans := api.Group("/taskPlans")
		{
			taskPlans.POST("/", taskplan.CreateTaskPlan)
			taskPlans.GET("/", taskplan.ListTaskPlans)
			taskPlans.GET("/tree", taskplan.GetTaskPlanTree)
			taskPlans.GET("/:id", taskplan.GetTaskPlan)
			taskPlans.PATCH("/:id", taskplan.UpdateTaskPlan)
			taskPlans.PATCH("/:id/start", taskplan.StartTaskPlan)
			taskPlans.PATCH("/:id/complete", taskplan.CompleteTaskPlan)
			taskPlans.PATCH("/:id/archive", taskplan.ArchiveTaskPlan)
			taskPlans.PATCH("/:id/suspend", taskplan.SuspendTaskPlan)
			taskPlans.PATCH("/:id/resume", taskplan.ResumeTaskPlan)
			taskPlans.PATCH("/:id/priority", taskplan.SetPriorityTaskPlan)
			taskPlans.DELETE("/:id", taskplan.DeleteTaskPlan)
			taskPlans.GET("/:id/tasks", taskplan.ListPlanTasks)
			taskPlans.GET("/:id/raw", taskplan.GetTaskPlanRaw)
			taskPlans.POST("/:id/review", taskplan.ReviewTaskPlan)
		}

		ai := api.Group("/ai")
		{
			ai.POST("/review-score", taskplan.AiReviewScore)
		}

		tasks := api.Group("/tasks")
		{
			tasks.GET("/pending", taskplan.GetPendingTasks)
			tasks.GET("/dailyStats", taskplan.GetTaskDailyStats)
			tasks.GET("/activeStats", taskplan.GetTaskActiveStats)
			tasks.GET("/contributionStats", taskplan.GetTaskContributionStats)
			tasks.PATCH("/:id/complete", taskplan.CompleteTask)
			tasks.PATCH("/:id/cancel", taskplan.CancelTask)
			tasks.PATCH("/:id/postpone", taskplan.PostponeTask)
			tasks.POST("/batch-postpone", taskplan.BatchPostponeTasks)
		}

		// 定时任务（独立模块，与任务计划 / 待办任务无关）：cron 调度 + 动作执行 + 运行日志
		cronJobs := api.Group("/cronJobs")
		{
			cronJobs.GET("/", cronjob.ListCronJobs)
			cronJobs.POST("/", cronjob.CreateCronJob)
			cronJobs.GET("/:id", cronjob.GetCronJob)
			cronJobs.PATCH("/:id", cronjob.UpdateCronJob)
			cronJobs.DELETE("/:id", cronjob.DeleteCronJob)
			cronJobs.PATCH("/:id/toggle", cronjob.ToggleCronJob)
			cronJobs.POST("/:id/run", cronjob.RunCronJob)
			cronJobs.GET("/:id/runs", cronjob.ListCronJobRuns)
			cronJobs.POST("/preview", cronjob.PreviewCron)
		}

		// 由原系统内置周期任务改造而来的内部动作接口，供定时任务模块通过 HTTP 触发
		sysJobs := api.Group("/systemJobs")
		{
			sysJobs.POST("/sweep", func(c *gin.Context) {
				taskplan.SweepScheduledTaskPlans()
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})
			sysJobs.POST("/portForwardSelfHeal", func(c *gin.Context) {
				portfwd.SelfHealForwards()
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})
			sysJobs.POST("/stunRuleSync", func(c *gin.Context) {
				stunsync.RunSync()
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})
			sysJobs.POST("/stunPortForwardSync", func(c *gin.Context) {
				stunpf.RunSync()
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})
		}

		notify := api.Group("/notify")
		{
			notify.POST("/", notifysvc.ShowNotify)
		}

		projectGroups := api.Group("/projectGroups")
		{
			projectGroups.POST("/", project.CreateProjectGroup)
			projectGroups.GET("/", project.ListProjectGroups)
			projectGroups.GET("/:id", project.GetProjectGroup)
			projectGroups.PATCH("/:id", project.UpdateProjectGroup)
			projectGroups.DELETE("/:id", project.DeleteProjectGroup)
		}

		projects := api.Group("/projects")
		{
			projects.POST("/", project.CreateProject)
			projects.GET("/", project.ListProjects)
			projects.GET("/:id", project.GetProject)
			projects.PATCH("/:id", project.UpdateProject)
			projects.PATCH("/:id/move", project.MoveProject)
			projects.PATCH("/:id/access", project.AccessProject)
			projects.DELETE("/:id", project.DeleteProject)
		}

		api.GET("/balance/deepseek", proxy.GetDeepSeekBalance)
		api.GET("/weather", proxy.GetWeather)
		api.GET("/hostname", proxy.GetHostname)

		// SSH 连接信息（凭据仅后端使用，响应一律脱敏）
		api.GET("/sshConns", portfwd.GetSshConnections)
		api.POST("/sshConns", portfwd.CreateSshConnection)
		api.PATCH("/sshConns/:id", portfwd.UpdateSshConnection)
		api.DELETE("/sshConns/:id", portfwd.DeleteSshConnection)
		api.POST("/sshConns/:id/test", portfwd.TestSshConnection)

		// SSH 隧道端口转发规则
		api.GET("/portForwards", portfwd.GetPortForwardings)
		api.POST("/portForwards", portfwd.CreatePortForwarding)
		api.PATCH("/portForwards/:id", portfwd.UpdatePortForwarding)
		api.DELETE("/portForwards/:id", portfwd.DeletePortForwarding)
		api.PATCH("/portForwards/:id/status", portfwd.UpdatePortForwardingStatus)
	}

	// 数据库监控（只读自省：表名 / 大小 / 行数 / 索引等统计）
	dbMon := api.Group("/dbMonitor")
	{
		dbMon.GET("/overview", dbmonitor.GetOverview)
		dbMon.GET("/tables", dbmonitor.ListTables)
		dbMon.GET("/tables/:name", dbmonitor.GetTableDetail)
	}

	r.GET("/", func(c *gin.Context) {
		f, err := webFS.Open("index.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	r.GET("/assets/*filepath", func(c *gin.Context) {
		filePath := c.Param("filepath")
		if strings.HasPrefix(filePath, "/") {
			filePath = strings.TrimPrefix(filePath, "/")
		}
		f, err := webFS.Open("assets/" + filePath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		c.Data(http.StatusOK, getContentType(filePath), content)
	})

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/debug/") {
			return
		}
		f, err := webFS.Open("index.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	return r
}

func getContentType(filename string) string {
	switch filepath.Ext(filename) {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".woff", ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}
