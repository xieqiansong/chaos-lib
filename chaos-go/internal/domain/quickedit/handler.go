package quickedit

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/quick-edits")
	g.POST("/list", func(c *web.Context) {
		items, err := ListFilesWithSnapshot()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
			return
		}
		renv.Success(c, items)
	})
	g.POST("/create", func(c *web.Context) {
		var req struct {
			Name     string `json:"name"`
			FilePath string `json:"filePath"`
			Remark   string `json:"remark"`
		}
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		file, snapshot, err := RegisterFile(req.Name, req.FilePath, req.Remark)
		if err != nil {
			quickEditErr(c, err)
			return
		}
		resp := buildFileResponse(*file)
		renv.Success(c, map[string]any{"message": "创建成功", "data": resp, "firstSnapshotId": snapshot.ID})
	})
	g.POST("/delete", func(c *web.Context) {
		id, ok := envelope.ID(c)
		if !ok {
			renv.Error(c, http.StatusBadRequest, "缺少 id")
			return
		}
		if err := DeleteFileByID(id); err != nil {
			quickEditErr(c, err)
			return
		}
		renv.Success(c, nil)
	})
	g.POST("/getContent", func(c *web.Context) {
		id, ok := envelope.ID(c)
		if !ok {
			renv.Error(c, http.StatusBadRequest, "缺少 id")
			return
		}
		view, err := ReadContent(id)
		if err != nil {
			httpx.MapError(c, err, errRules)
			return
		}
		renv.Success(c, view)
	})
	g.POST("/updateContent", func(c *web.Context) {
		id, ok := envelope.ID(c)
		if !ok {
			renv.Error(c, http.StatusBadRequest, "缺少 id")
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		_ = envelope.Bind(c, &req)
		res, err := SaveContent(id, req.Content)
		if err != nil {
			httpx.MapError(c, err, errRules)
			return
		}
		renv.Success(c, saveResultToMap(res, false))
	})
	g.POST("/listSnapshots", func(c *web.Context) {
		id, ok := envelope.ID(c)
		if !ok {
			renv.Error(c, http.StatusBadRequest, "缺少 id")
			return
		}
		var m struct {
			Page     int `json:"page"`
			PageSize int `json:"pageSize"`
		}
		envelope.GetMeta(c, &m)
		q := pagination.Query{Page: m.Page, PageSize: m.PageSize}
		if q.Page < 1 {
			q.Page = pagination.DefaultPage
		}
		if q.PageSize < 1 || q.PageSize > pagination.MaxPageSize {
			q.PageSize = pagination.DefaultPageSize
		}
		items, total, err := ListSnapshotsOf(id, q)
		if err != nil {
			httpx.MapError(c, err, errRules)
			return
		}
		renv.Success(c, pagination.New(items, total, q))
	})
	g.POST("/getSnapshot", func(c *web.Context) {
		var req struct {
			FileID     int `json:"fileId"`
			SnapshotID int `json:"snapshotId"`
		}
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		snap, err := GetSnapshot(req.FileID, req.SnapshotID)
		if err != nil {
			httpx.MapError(c, err, errRules)
			return
		}
		renv.Success(c, map[string]any{
			"id":        snap.ID,
			"fileId":    snap.FileID,
			"content":   snap.Content,
			"sizeBytes": snap.SizeBytes,
			"createdAt": snap.CreatedAt,
		})
	})
	g.POST("/restore", func(c *web.Context) {
		var req struct {
			FileID     int `json:"fileId"`
			SnapshotID int `json:"snapshotId"`
		}
		if err := envelope.Bind(c, &req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		res, err := RestoreSnapshot(req.FileID, req.SnapshotID)
		if err != nil {
			httpx.MapError(c, err, errRules)
			return
		}
		renv.Success(c, saveResultToMap(res, true))
	})
}

func quickEditErr(c *web.Context, err error) {
	switch {
	case errors.Is(err, ErrFileExists), errors.Is(err, ErrPathNotFound),
		errors.Is(err, ErrInvalidPath), errors.Is(err, ErrContentTooLarge):
		renv.Error(c, http.StatusBadRequest, err.Error())
	default:
		renv.Error(c, http.StatusInternalServerError, err.Error())
	}
}

func ListQuickEdits(c *web.Context) {
	items, err := ListFilesWithSnapshot()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, items)
}

func CreateQuickEdit(c *web.Context) {
	var req struct {
		Name     string
		FilePath string
		Remark   string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	file, snapshot, err := RegisterFile(req.Name, req.FilePath, req.Remark)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileExists), errors.Is(err, ErrPathNotFound),
			errors.Is(err, ErrInvalidPath), errors.Is(err, ErrContentTooLarge):
			renv.Error(c, http.StatusBadRequest, err.Error())
		default:
			renv.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	resp := buildFileResponse(*file)
	renv.Success(c, map[string]any{
		"message":         "创建成功",
		"data":            resp,
		"firstSnapshotId": snapshot.ID,
	})
}

func DeleteQuickEdit(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	if err := DeleteFileByID(id); err != nil {
		if errors.Is(err, ErrFileNotFound) {
			renv.Error(c, http.StatusNotFound, "文件不存在")
			return
		}
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, nil)
}

func GetQuickEditContent(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	view, err := ReadContent(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, view)
}

func UpdateQuickEditContent(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	var req struct{ Content string }
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	res, err := SaveContent(id, req.Content)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, saveResultToMap(res, false))
}

func ListQuickEditSnapshots(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	q := pagination.Parse(c)
	items, total, err := ListSnapshotsOf(id, q)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, pagination.New(items, total, q))
}

func GetQuickEditSnapshot(c *web.Context) {
	fileID, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	snapID, ok := httpx.ParseParam(c, "snapshotId")
	if !ok {
		return
	}
	snap, err := GetSnapshot(fileID, snapID)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, map[string]any{
		"id":        snap.ID,
		"fileId":    snap.FileID,
		"content":   snap.Content,
		"sizeBytes": snap.SizeBytes,
		"createdAt": snap.CreatedAt,
	})
}

func RestoreQuickEdit(c *web.Context) {
	fileID, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	var req struct{ SnapshotID int }
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	res, err := RestoreSnapshot(fileID, req.SnapshotID)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, saveResultToMap(res, true))
}

var errRules = []httpx.ErrRule{
	{Err: ErrFileNotFound, Status: http.StatusNotFound, Msg: "文件不存在"},
	{Err: ErrSnapshotNotFound, Status: http.StatusNotFound, Msg: "快照不存在"},
	{Err: ErrEnvNotReady, Status: http.StatusNotImplemented},
	{Err: ErrContentTooLarge, Status: http.StatusBadRequest},
}

func saveResultToMap(res *SaveResult, withFrom bool) map[string]any {
	out := map[string]any{
		"message":      res.Message,
		"data":         res.Data,
		"snapshotId":   res.SnapshotID,
		"snapshotTime": res.SnapshotTime,
	}
	if withFrom {
		out["fromSnapshotId"] = res.FromSnapshotID
	}
	if len(res.Warnings) > 0 {
		out["warnings"] = res.Warnings
	}
	return out
}
