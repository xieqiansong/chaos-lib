package models

type FileLink struct {
	ID         int    `gorm:"primaryKey"`
	SourcePath string ``
	TargetPath string ``
	Status     bool   ``
	Remark     string ``
	Sort       int    `gorm:"default:0"`
}

type FileLinkResponse struct {
	ID         int
	SourcePath string
	TargetPath string
	Status     bool
	Remark     string
	Sort       int
	LinkStatus string
}
