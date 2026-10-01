package quickedit

import "time"

// QuickEditFileResponse 文件列表/详情响应（附带最近一次快照信息）。
type QuickEditFileResponse struct {
	ID               int
	Name             string
	FilePath         string
	Remark           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastSnapshotID   int
	LastSnapshotTime time.Time
}

// QuickEditSnapshotResponse 快照列表项。
type QuickEditSnapshotResponse struct {
	ID        int
	FileID    int
	SizeBytes int
	CreatedAt time.Time
}

// ContentView 文件内容视图。
type ContentView struct {
	Content   string `json:"content"`
	FilePath  string `json:"filePath"`
	SizeBytes int    `json:"sizeBytes"`
}

// SaveResult 保存 / 回滚的结果：新快照 + 富化后的文件信息 + 非致命告警。
// FromSnapshotID 仅回滚场景使用（来源快照）。
type SaveResult struct {
	Message        string
	Data           *QuickEditFileResponse
	SnapshotID     int
	SnapshotTime   time.Time
	FromSnapshotID int
	Warnings       []string
}

// buildFileResponse 富化文件响应：补上最近一次快照信息。
func buildFileResponse(file QuickEditFile) QuickEditFileResponse {
	resp := QuickEditFileResponse{
		ID:        file.ID,
		Name:      file.Name,
		FilePath:  file.FilePath,
		Remark:    file.Remark,
		CreatedAt: file.CreatedAt,
		UpdatedAt: file.UpdatedAt,
	}
	if latest, err := GetLatestSnapshot(file.ID); err == nil && latest.ID > 0 {
		resp.LastSnapshotID = latest.ID
		resp.LastSnapshotTime = latest.CreatedAt
	}
	return resp
}

// toSnapshotResponses 把快照列表转换为列表项视图。
func toSnapshotResponses(snaps []QuickEditSnapshot) []QuickEditSnapshotResponse {
	items := make([]QuickEditSnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		items = append(items, QuickEditSnapshotResponse{ID: s.ID, FileID: s.FileID, SizeBytes: s.SizeBytes, CreatedAt: s.CreatedAt})
	}
	return items
}
