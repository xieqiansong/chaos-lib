package envvar

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("envVariables", Register)
}

// Register 挂载环境变量相关路由（资源接口自包含，router.go 仅做编排调用）。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/envVariables")
	g.GET("/", GetEnvVariables)
	g.PATCH("/", PatchEnvVariables)
	g.PUT("/", PutEnvVariables)
	g.POST("/sync", SyncEnvVariables)
	g.GET("/snapshots/:snapshotId", GetEnvSnapshotDetail)
}

// GetEnvVariables 返回当前系统 / 用户环境变量与最近一次快照信息。
func GetEnvVariables(c *gin.Context) {
	resp, err := Load()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, resp)
}

// SyncEnvVariables 以当前系统变量为准重落一条快照。
func SyncEnvVariables(c *gin.Context) {
	warnings, err := Sync()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	logWarnings("同步环境变量", warnings)
	renv.Success(c, nil)
}

// PatchEnvVariables 按增量补丁（Set / Unset / Path 增删改）局部改写环境变量。
func PatchEnvVariables(c *gin.Context) {
	var req EnvPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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
}

// PutEnvVariables 整体替换 system / 用户两段环境变量。
func PutEnvVariables(c *gin.Context) {
	var req EnvPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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
}

// GetEnvSnapshotDetail 返回指定快照的内容与解析结果。
func GetEnvSnapshotDetail(c *gin.Context) {
	snapID, err := strconv.Atoi(c.Param("snapshotId"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 snapshot id")
		return
	}
	detail, err := GetSnapshotDetail(snapID)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			renv.Error(c, http.StatusNotFound, "快照不存在")
			return
		}
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, detail)
}

// logWarnings 记录写入 / 快照过程中的非致命告警。
// 响应体保持既有形状（data 为 null），告警只进日志，避免改动对外契约。
func logWarnings(scope string, warnings []string) {
	for _, w := range warnings {
		slog.Warn("环境变量操作告警", "scope", scope, "warn", w)
	}
}
