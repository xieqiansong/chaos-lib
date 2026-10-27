package dict

import (
	"chaos-go/internal/framework/crud"
)

type DictType struct {
	crud.BaseModel
	Name   string `json:"Name" gorm:"size:50;not null"`
	Code   string `json:"Code" gorm:"size:64;not null;index"`
	Status bool   `json:"Status" gorm:"default:true"`
	Sort   int    `json:"Sort" gorm:"default:0"`
	Remark string `json:"Remark" gorm:"size:255"`
}

func (DictType) TableName() string { return "dict_type" }

type DictData struct {
	crud.BaseModel
	TypeCode string `json:"TypeCode" gorm:"size:64;not null;index"`
	Label    string `json:"Label" gorm:"size:100;not null"`
	Value    string `json:"Value" gorm:"size:100;not null"`
	ParentID int    `json:"ParentID" gorm:"default:0"`
	Sort     int    `json:"Sort" gorm:"default:0"`
	Status   bool   `json:"Status" gorm:"default:true"`
	Remark   string `json:"Remark" gorm:"size:255"`
	Ext      string `json:"Ext" gorm:"type:text"`
}

func (DictData) TableName() string { return "dict_data" }
