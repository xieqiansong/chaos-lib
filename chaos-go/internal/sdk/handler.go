package sdk

import (
	"encoding/json"
	"errors"
	"net/http"

	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// GetSdkVersions 返回所有启用 SDK 类型的版本信息，map key = 类型 Name。
func GetSdkVersions(c *gin.Context) {
	srcs, err := ListActiveSdkSources()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	result := make(map[string]SdkInfo)
	for _, s := range srcs {
		if !s.Enabled {
			continue
		}
		result[s.Name] = getSdkInfo(s)
	}
	renv.Success(c, result)
}

// GetSdkVersion 返回指定类型的版本信息。
func GetSdkVersion(c *gin.Context) {
	typ := c.Param("type")
	s, err := FindSdkSource(typ)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "SDK type not found")
		return
	}
	renv.Success(c, getSdkInfo(*s))
}

// UpdateSdkVersion 切换版本：仅对 repo 来源生效。
func UpdateSdkVersion(c *gin.Context) {
	typ := c.Param("type")
	s, err := FindSdkSource(typ)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "SDK type not found")
		return
	}
	var req struct {
		Version string
	}
	if err := c.BindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if err := SwitchSdkVersion(*s, req.Version); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			renv.Error(c, http.StatusBadRequest, "Target version does not exist: "+req.Version)
			return
		}
		renv.Error(c, http.StatusInternalServerError, "Failed to switch version: "+err.Error()+". (Maybe need admin rights?)")
		return
	}
	renv.Success(c, nil)
}

// ── SDK 类型 CRUD ──

// ListSdkSources 列出所有未删除的 SDK 类型。
func ListSdkSources(c *gin.Context) {
	srcs, err := ListActiveSdkSources()
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
	if req.Name == "" {
		renv.Error(c, http.StatusBadRequest, "Name is required")
		return
	}
	for _, it := range parseSources(req.Sources) {
		if it.Kind != "repo" && it.Kind != "single" {
			renv.Error(c, http.StatusBadRequest, "Source kind must be 'repo' or 'single'")
			return
		}
		if it.Root == "" {
			renv.Error(c, http.StatusBadRequest, "Source root is required")
			return
		}
	}
	count, err := CountActiveByName(req.Name)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		renv.Error(c, http.StatusConflict, "SDK type already exists: "+req.Name)
		return
	}
	if err := CreateSdkSourceRow(&req); err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, req)
}

// UpdateSdkSource 编辑 SDK 类型（Sources/Current/Enabled/Note）。
func UpdateSdkSource(c *gin.Context) {
	name := c.Param("name")
	s, err := FindSdkSource(name)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "SDK type not found")
		return
	}
	var patch struct {
		Sources json.RawMessage
		Current string
		Enabled *bool
		Note    string
	}
	if err := c.ShouldBindJSON(&patch); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	updates := map[string]interface{}{}
	if patch.Sources != nil {
		for _, it := range parseSources(patch.Sources) {
			if it.Kind != "repo" && it.Kind != "single" {
				renv.Error(c, http.StatusBadRequest, "Source kind must be 'repo' or 'single'")
				return
			}
			if it.Root == "" {
				renv.Error(c, http.StatusBadRequest, "Source root is required")
				return
			}
		}
		updates["sources"] = patch.Sources
	}
	if patch.Current != "" {
		updates["current"] = patch.Current
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if patch.Note != "" || len(patch.Sources) > 0 {
		updates["note"] = patch.Note
	}
	if len(updates) == 0 {
		renv.Success(c, s)
		return
	}
	if err := ApplySdkSourcePatch(name, updates); err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, s)
}

// DeleteSdkSource 软删除 SDK 类型。
func DeleteSdkSource(c *gin.Context) {
	name := c.Param("name")
	if _, err := FindSdkSource(name); err != nil {
		renv.Error(c, http.StatusNotFound, "SDK type not found")
		return
	}
	if err := SoftDeleteSdkSource(name); err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, nil)
}

// ── 路由注册（自包含）────────────────────────────────────────────

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
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
