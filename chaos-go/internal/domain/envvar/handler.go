package envvar

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/env-variables")
	g.POST("/get", func(c *web.Context) {
		var req EnvGetRequest
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		resp, err := Load(req.Scope)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		renv.Success(c, resp)
	})
	g.POST("/patch", func(c *web.Context) {
		t0 := time.Now()
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
		slog.Info("[envvar] PATCH /env-variables/patch 端到端耗时", "ms", time.Since(t0).Milliseconds(), "hasErr", err != nil)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		logWarnings("局部更新环境变量", warnings)
		renv.Success(c, nil)
	})
	g.POST("/put", func(c *web.Context) {
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
	g.POST("/sync", func(c *web.Context) {
		warnings, err := Sync()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		logWarnings("同步环境变量", warnings)
		renv.Success(c, nil)
	})
	g.POST("/getSnapshot", func(c *web.Context) {
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

func logWarnings(scope string, warnings []string) {
	for _, w := range warnings {
		slog.Warn("环境变量操作告警", "scope", scope, "warn", w)
	}
}
