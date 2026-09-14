package proxy

import (
	"net/http"
	"os"
	"os/user"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetHostname 返回运行服务端的主机名与启动用户，供前端用作浏览器标签标题。
func GetHostname(c *gin.Context) {
	host, err := os.Hostname()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取主机名失败: " + err.Error()})
		return
	}
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	// Windows 下 Username 形如 "域名\\用户名"（如 WIN11-HP\xqs），
	// 只取最后一段，避免标题里出现重复的机器名前缀。
	if idx := strings.LastIndex(username, "\\"); idx >= 0 {
		username = username[idx+1:]
	}
	c.JSON(http.StatusOK, gin.H{"hostname": host, "username": username})
}
