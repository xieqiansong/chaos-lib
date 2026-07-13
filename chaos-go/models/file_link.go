package models

type FileLink struct {
	ID         int    `gorm:"primaryKey"`
	SourcePath string ``
	TargetPath string ``
	Status     bool   ``
	Remark     string ``
}

type FileLinkResponse struct {
	ID         int    ``
	SourcePath string ``
	TargetPath string ``
	Status     bool   ``
	Remark     string ``
	LinkStatus string ``
}
