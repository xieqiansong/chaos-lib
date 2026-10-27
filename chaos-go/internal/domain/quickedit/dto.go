package quickedit

import "time"

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

type QuickEditSnapshotResponse struct {
	ID        int
	FileID    int
	SizeBytes int
	CreatedAt time.Time
}

type ContentView struct {
	Content   string `json:"content"`
	FilePath  string `json:"filePath"`
	SizeBytes int    `json:"sizeBytes"`
}

type SaveResult struct {
	Message        string
	Data           *QuickEditFileResponse
	SnapshotID     int
	SnapshotTime   time.Time
	FromSnapshotID int
	Warnings       []string
}

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

func toSnapshotResponses(snaps []QuickEditSnapshot) []QuickEditSnapshotResponse {
	items := make([]QuickEditSnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		items = append(items, QuickEditSnapshotResponse{ID: s.ID, FileID: s.FileID, SizeBytes: s.SizeBytes, CreatedAt: s.CreatedAt})
	}
	return items
}
