package notify

import (
	renv "chaos-go/internal/resp"
	"log/slog"

	"github.com/gin-gonic/gin"
)

func ShowNotify(c *gin.Context) {
	var req struct {
		Title   string
		Content string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, 400, err.Error())
		return
	}

	ShowWindowsNotify(req.Title, req.Content)
	slog.Info("通知已发送", "title", req.Title, "content", req.Content)
	renv.Success(c, nil)
}
