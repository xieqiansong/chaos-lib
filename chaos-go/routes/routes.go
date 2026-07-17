package routes

import (
	"chaos-go/services"
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
		api.GET("/browserHistories", services.GetBrowserHistories)
		api.POST("/browserHistories", services.SaveBrowserHistory)
		api.POST("/browserHistoryVisits", services.SaveBrowserHistoryVisits)

		api.GET("/sdks", services.GetSdkVersions)
		api.GET("/sdks/:type", services.GetSdkVersion)
		api.PATCH("/sdks/:type/switch", services.UpdateSdkVersion)

		api.GET("/fileLinks", services.GetFileLinks)
		api.POST("/fileLinks", services.CreateFileLink)
		api.DELETE("/fileLinks/:id", services.DeleteFileLink)
		api.PATCH("/fileLinks/:id", services.UpdateFileLink)
		api.PATCH("/fileLinks/:id/status", services.UpdateFileLinkStatus)

		quickEdits := api.Group("/quickEdits")
		{
			quickEdits.GET("/", services.ListQuickEdits)
			quickEdits.POST("/", services.CreateQuickEdit)
			quickEdits.DELETE("/:id", services.DeleteQuickEdit)
			quickEdits.GET("/:id/content", services.GetQuickEditContent)
			quickEdits.PUT("/:id/content", services.UpdateQuickEditContent)
			quickEdits.GET("/:id/snapshots", services.ListQuickEditSnapshots)
			quickEdits.GET("/:id/snapshots/:snapshotId", services.GetQuickEditSnapshot)
			quickEdits.POST("/:id/restore", services.RestoreQuickEdit)
		}

		envVars := api.Group("/envVariables")
		{
			envVars.GET("/", services.GetEnvVariables)
			envVars.PATCH("/", services.PatchEnvVariables)
			envVars.PUT("/", services.PutEnvVariables)
			envVars.POST("/sync", services.SyncEnvVariables)
			envVars.GET("/snapshots/:snapshotId", services.GetEnvSnapshotDetail)
		}

		taskPlans := api.Group("/taskPlans")
		{
			taskPlans.POST("/", services.CreateTaskPlan)
			taskPlans.GET("/", services.ListTaskPlans)
			taskPlans.GET("/tree", services.GetTaskPlanTree)
			taskPlans.GET("/:id", services.GetTaskPlan)
			taskPlans.PATCH("/:id", services.UpdateTaskPlan)
			taskPlans.PATCH("/:id/start", services.StartTaskPlan)
			taskPlans.PATCH("/:id/complete", services.CompleteTaskPlan)
			taskPlans.PATCH("/:id/archive", services.ArchiveTaskPlan)
			taskPlans.PATCH("/:id/suspend", services.SuspendTaskPlan)
			taskPlans.PATCH("/:id/resume", services.ResumeTaskPlan)
			taskPlans.PATCH("/:id/priority", services.SetPriorityTaskPlan)
			taskPlans.DELETE("/:id", services.DeleteTaskPlan)
			taskPlans.GET("/:id/tasks", services.ListPlanTasks)
		}

		tasks := api.Group("/tasks")
		{
			tasks.GET("/pending", services.GetPendingTasks)
			tasks.GET("/dailyStats", services.GetTaskDailyStats)
			tasks.GET("/activeStats", services.GetTaskActiveStats)
			tasks.PATCH("/:id/complete", services.CompleteTask)
			tasks.PATCH("/:id/cancel", services.CancelTask)
			tasks.PATCH("/:id/postpone", services.PostponeTask)
		}

		notify := api.Group("/notify")
		{
			notify.POST("/", services.ShowNotify)
		}

		// 本地项目文件夹管理
		projectGroups := api.Group("/projectGroups")
		{
			projectGroups.POST("/", services.CreateProjectGroup)
			projectGroups.GET("/", services.ListProjectGroups)
			projectGroups.GET("/:id", services.GetProjectGroup)
			projectGroups.PATCH("/:id", services.UpdateProjectGroup)
			projectGroups.DELETE("/:id", services.DeleteProjectGroup)
		}

		projects := api.Group("/projects")
		{
			projects.POST("/", services.CreateProject)
			projects.GET("/", services.ListProjects)
			projects.GET("/:id", services.GetProject)
			projects.PATCH("/:id", services.UpdateProject)
			projects.PATCH("/:id/move", services.MoveProject)
			projects.PATCH("/:id/access", services.AccessProject)
			projects.DELETE("/:id", services.DeleteProject)
			projects.POST("/:id/restore", services.RestoreProject)
		}

		api.GET("/balance/deepseek", services.GetDeepSeekBalance)
		api.GET("/weather", services.GetWeather)
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
