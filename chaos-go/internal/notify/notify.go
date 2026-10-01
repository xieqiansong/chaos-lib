package notify

import (
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"
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

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("notify", Register)
}

// Register 挂载系统通知接口。
func Register(rg *gin.RouterGroup) {
	rg.Group("/notify").POST("/", ShowNotify)
}
