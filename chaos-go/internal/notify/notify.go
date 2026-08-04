package notify

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

func ShowNotify(c *gin.Context) {
	var req struct {
		Title   string
		Content string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ShowWindowsNotify(req.Title, req.Content)
	slog.Info("通知已发送", "title", req.Title, "content", req.Content)
	c.JSON(200, gin.H{"result": "sent", "message": "已发送通知"})
}
