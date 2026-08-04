package routes

import (
	"chaos-go/internal/envvar"
	"chaos-go/internal/filelink"
	notifysvc "chaos-go/internal/notify"
	"chaos-go/internal/project"
	"chaos-go/internal/proxy"
	"chaos-go/internal/quickedit"
	"chaos-go/internal/taskplan"
	"embed"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetupRouter(webFS embed.FS) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/browserHistories", proxy.GetBrowserHistories)
		api.POST("/browserHistories", proxy.SaveBrowserHistory)
		api.POST("/browserHistoryVisits", proxy.SaveBrowserHistoryVisits)

		api.GET("/sdks", proxy.GetSdkVersions)
		api.GET("/sdks/:type", proxy.GetSdkVersion)
		api.PATCH("/sdks/:type/switch", proxy.UpdateSdkVersion)

		api.GET("/fileLinks", filelink.GetFileLinks)
		api.POST("/fileLinks", filelink.CreateFileLink)
		api.DELETE("/fileLinks/:id", filelink.DeleteFileLink)
		api.PATCH("/fileLinks/:id", filelink.UpdateFileLink)
		api.PATCH("/fileLinks/:id/status", filelink.UpdateFileLinkStatus)

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
		}

		tasks := api.Group("/tasks")
		{
			tasks.GET("/pending", taskplan.GetPendingTasks)
			tasks.GET("/dailyStats", taskplan.GetTaskDailyStats)
			tasks.GET("/activeStats", taskplan.GetTaskActiveStats)
			tasks.PATCH("/:id/complete", taskplan.CompleteTask)
			tasks.PATCH("/:id/cancel", taskplan.CancelTask)
			tasks.PATCH("/:id/postpone", taskplan.PostponeTask)
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
			projects.POST("/:id/restore", project.RestoreProject)
		}

		api.GET("/balance/deepseek", proxy.GetDeepSeekBalance)
		api.GET("/weather", proxy.GetWeather)
	}

	r.GET("/", func(c *gin.Context) {
		f, err := webFS.Open("web/index.html")
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
		f, err := webFS.Open("web/assets/" + filePath)
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
		f, err := webFS.Open("web/index.html")
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
