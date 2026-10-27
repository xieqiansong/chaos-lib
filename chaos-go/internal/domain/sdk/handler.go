package sdk

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/sdks")
	g.POST("/list", func(c *web.Context) {
		result, err := SdkVersions()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		renv.Success(c, result)
	})
	g.POST("/get", func(c *web.Context) {
		var req struct {
			Type string `json:"type"`
		}
		_ = envelope.Bind(c, &req)
		info, err := SdkVersion(req.Type)
		if err != nil {
			httpx.MapError(c, err, sdkErrRules)
			return
		}
		renv.Success(c, info)
	})
	g.POST("/switch", func(c *web.Context) {
		var req struct {
			Type    string `json:"type"`
			Version string `json:"version"`
		}
		_ = envelope.Bind(c, &req)
		if err := SwitchVersion(req.Type, req.Version); err != nil {
			if errors.Is(err, ErrVersionNotFound) {
				renv.Error(c, http.StatusBadRequest, "Target version does not exist: "+req.Version)
				return
			}
			httpx.MapError(c, err, sdkErrRules)
			return
		}
		renv.Success(c, nil)
	})
	g.POST("/listSources", func(c *web.Context) {
		srcs, err := ListSources()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		renv.Success(c, srcs)
	})
	g.POST("/createSource", func(c *web.Context) {
		var req SdkSource
		if err := envelope.Bind(c, &req); err != nil {
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
	})
	g.POST("/updateSource", func(c *web.Context) {
		var req struct {
			Name        string `json:"name"`
			SourcePatch SourcePatch
		}
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		updated, err := UpdateSource(req.Name, req.SourcePatch)
		if err != nil {
			httpx.MapError(c, err, sdkErrRules)
			return
		}
		renv.Success(c, updated)
	})
	g.POST("/deleteSource", func(c *web.Context) {
		var req struct {
			Name string `json:"name"`
		}
		_ = envelope.Bind(c, &req)
		if err := DeleteSource(req.Name); err != nil {
			httpx.MapError(c, err, sdkErrRules)
			return
		}
		renv.Success(c, nil)
	})
}

func GetSdkVersions(c *web.Context) {
	result, err := SdkVersions()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, result)
}

func GetSdkVersion(c *web.Context) {
	info, err := SdkVersion(c.Param("type"))
	if err != nil {
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, info)
}

func UpdateSdkVersion(c *web.Context) {
	var req struct {
		Version string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
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

func ListSdkSources(c *web.Context) {
	srcs, err := ListSources()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, srcs)
}

func CreateSdkSource(c *web.Context) {
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

func UpdateSdkSource(c *web.Context) {
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

func DeleteSdkSource(c *web.Context) {
	if err := DeleteSource(c.Param("name")); err != nil {
		httpx.MapError(c, err, sdkErrRules)
		return
	}
	renv.Success(c, nil)
}

var sdkErrRules = []httpx.ErrRule{
	{Err: ErrSdkNotFound, Status: http.StatusNotFound, Msg: "SDK type not found"},
	{Err: ErrInvalidInput, Status: http.StatusBadRequest},
	{Err: ErrNameRequired, Status: http.StatusBadRequest},
}
