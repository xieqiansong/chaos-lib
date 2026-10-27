package filelink

import "chaos-go/internal/framework/crud"

type FileLink struct {
	crud.BaseModel
	SourcePath string `json:"SourcePath"`
	TargetPath string `json:"TargetPath"`
	Status     bool   `json:"Status"`
	Remark     string `json:"Remark"`
	Sort       int    `gorm:"default:0" json:"Sort"`
}

func (FileLink) TableName() string { return "file_links" }
