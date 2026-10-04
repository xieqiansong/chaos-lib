package sdk

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("sdk", Register)
}

// Register 把 SDK 全部路由挂载到给定路由组。
// 注意：本模块未走通用 crud，由专用 handler 直接接线，以承载版本切换/软链等定制行为。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/sdks")
	g.GET("", GetSdkVersions)
	g.GET("/:type", GetSdkVersion)
	g.PATCH("/:type/switch", UpdateSdkVersion)
	g.GET("/defs", ListSdkSources)
	g.POST("/defs", CreateSdkSource)
	g.PATCH("/defs/:name", UpdateSdkSource)
	g.DELETE("/defs/:name", DeleteSdkSource)
}

// GetSdkVersions 返回所有启用 SDK 类型的版本信息，map key = 类型 Name。
func GetSdkVersions(c *gin.Context) {
	result, err := SdkVersions()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, result)
}

// GetSdkVersion 返回指定类型的版本信息。
func GetSdkVersion(c *gin.Context) {
	info, err := SdkVersion(c.Param("type"))
	if err != nil {
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, info)
}

// UpdateSdkVersion 切换版本：仅对 repo 来源生效。
func UpdateSdkVersion(c *gin.Context) {
	var req struct {
		Version string
	}
	if err := c.BindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if err := SwitchVersion(c.Param("type"), req.Version); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			renv.Error(c, http.StatusBadRequest, "Target version does not exist: "+req.Version)
			return
		}
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, nil)
}

// ── SDK 类型 CRUD ──

// ListSdkSources 列出所有未删除的 SDK 类型。
func ListSdkSources(c *gin.Context) {
	srcs, err := ListSources()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, srcs)
}

// CreateSdkSource 新增 SDK 类型。
func CreateSdkSource(c *gin.Context) {
	var req SdkSource
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := CreateSource(&req); err != nil {
		if errors.Is(err, ErrSourceExists) {
			renv.Error(c, http.StatusConflict, err.Error())
			return
		}
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, req)
}

// UpdateSdkSource 编辑 SDK 类型（Sources/Current/Enabled/Note）。
func UpdateSdkSource(c *gin.Context) {
	var patch SourcePatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := UpdateSource(c.Param("name"), patch)
	if err != nil {
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, updated)
}

// DeleteSdkSource 软删除 SDK 类型。
func DeleteSdkSource(c *gin.Context) {
	if err := DeleteSource(c.Param("name")); err != nil {
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, nil)
}

// sdkErrRules 领域错误 → HTTP 状态码映射表，取代原先内联在 handler 里的 writeError。
var sdkErrRules = []httpx.ErrRule{
	{Err: ErrSdkNotFound, Status: http.StatusNotFound, Msg: "SDK type not found"},
	{Err: ErrInvalidInput, Status: http.StatusBadRequest},
	{Err: ErrNameRequired, Status: http.StatusBadRequest},
}
