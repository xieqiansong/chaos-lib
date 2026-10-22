package dict

import (
	"chaos-go/internal/framework/crud"
)

// DictType 字典类型：一组枚举的容器，由业务模块通过 Code 引用。
// 例如 存放位置(item_location) / 物品分类(item_category) / 购入来源(purchase_source)。
type DictType struct {
	crud.BaseModel
	Name   string `json:"Name" gorm:"size:50;not null"`
	Code   string `json:"Code" gorm:"size:64;not null;index"`
	Status bool   `json:"Status" gorm:"default:true"`
	Sort   int    `json:"Sort" gorm:"default:0"`
	Remark string `json:"Remark" gorm:"size:255"`
}

func (DictType) TableName() string { return "dict_type" }

// DictData 字典数据项：某一 DictType 下的具体可选项；ParentID 支持父子级（如 家 > 书房）。
// Label 给用户看，Value 存进业务表；其他模块统一按 TypeCode + Value 引用。
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
