package filelink

import "time"

// FileLinkResponse 对外响应：模型字段之外附带实时计算的 LinkStatus。
type FileLinkResponse struct {
	ID         int       `json:"ID"`
	SourcePath string    `json:"SourcePath"`
	TargetPath string    `json:"TargetPath"`
	Status     bool      `json:"Status"`
	Remark     string    `json:"Remark"`
	Sort       int       `json:"Sort"`
	LinkStatus string    `json:"LinkStatus"`
	CreatedAt  time.Time `json:"CreatedAt"`
	UpdatedAt  time.Time `json:"UpdatedAt"`
}

// toOne 单条转换：LinkStatus 由 service 按文件系统实时推导。
func toOne(l *FileLink) FileLinkResponse {
	return FileLinkResponse{
		ID:         l.ID,
		SourcePath: l.SourcePath,
		TargetPath: l.TargetPath,
		Status:     l.Status,
		Remark:     l.Remark,
		Sort:       l.Sort,
		LinkStatus: checkLinkStatus(l.SourcePath, l.TargetPath, l.Status),
		CreatedAt:  l.CreatedAt,
		UpdatedAt:  l.UpdatedAt,
	}
}

// toResponse 整批转换（crud.Opts.ToResponse）：入参固定 []*FileLink。
func toResponse(rows []*FileLink) any {
	out := make([]FileLinkResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, toOne(l))
	}
	return out
}
