package envvar

import (
	"errors"
	"log/slog"
	"net/http"

	"chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由已废弃（前端统一走 /api/v1 动作路由），此处仅保留 RegisterV1。
func init() {
	routehub.RegisterV1("env-variables", RegisterV1)
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 动作：get / patch / put / sync / getSnapshot；补丁与快照 id 取自信封 data。
func RegisterV1(rg *gin.RouterGroup) {
	g := rg.Group("/env-variables")
	g.POST("/get", func(c *gin.Context) {
		resp, err := Load()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		renv.Success(c, resp)
	})
	g.POST("/patch", func(c *gin.Context) {
		var req EnvPatchRequest
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		if req.System == nil && req.User == nil {
			renv.Error(c, http.StatusBadRequest, "请求体不能为空（至少需要 system 或 user 之一）")
			return
		}
		warnings, err := ApplyPatch(&req)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		logWarnings("局部更新环境变量", warnings)
		renv.Success(c, nil)
	})
	g.POST("/put", func(c *gin.Context) {
		var req EnvPutRequest
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		warnings, err := ReplaceAll(&req)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		logWarnings("整体替换环境变量", warnings)
		renv.Success(c, nil)
	})
	g.POST("/sync", func(c *gin.Context) {
		warnings, err := Sync()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		logWarnings("同步环境变量", warnings)
		renv.Success(c, nil)
	})
	g.POST("/getSnapshot", func(c *gin.Context) {
		var req struct {
			SnapshotID int `json:"snapshotId"`
		}
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		detail, err := GetSnapshotDetail(req.SnapshotID)
		if err != nil {
			if errors.Is(err, ErrSnapshotNotFound) {
				renv.Error(c, http.StatusNotFound, "快照不存在")
				return
			}
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		renv.Success(c, detail)
	})
}

// logWarnings 记录写入 / 快照过程中的非致命告警。
// 响应体保持既有形状（data 为 null），告警只进日志，避免改动对外契约。
func logWarnings(scope string, warnings []string) {
	for _, w := range warnings {
		slog.Warn("环境变量操作告警", "scope", scope, "warn", w)
	}
}
