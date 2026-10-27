package proxy

import (
	renv "chaos-go/internal/framework/resp"
	"net/http"
	"os"
	"os/user"
	"strings"

	"chaos-go/internal/framework/web"
)

func GetHostname(c *web.Context) {
	host, err := os.Hostname()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "获取主机名失败: "+err.Error())
		return
	}
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	if idx := strings.LastIndex(username, "\\"); idx >= 0 {
		username = username[idx+1:]
	}
	renv.Success(c, map[string]any{"hostname": host, "username": username})
}
