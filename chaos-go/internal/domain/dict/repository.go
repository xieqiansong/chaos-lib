package dict

import (
	"errors"

	"chaos-go/internal/framework/config"

	"gorm.io/gorm"
)

// 领域哨兵错误（供 service / handler 映射为 HTTP 状态码）。
var (
	ErrDBUnavailable    = errors.New("dict: database unavailable")
	ErrDictTypeNotFound = errors.New("dict: 字典类型不存在")
	ErrDictDataNotFound = errors.New("dict: 字典数据不存在")
)

// FindActiveDictTypeByID 按 ID 加载未删除的字典类型；不存在返回 ErrDictTypeNotFound。
func FindActiveDictTypeByID(id string) (*DictType, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var row DictType
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDictTypeNotFound
		}
		return nil, err
	}
	return &row, nil
}

// SetDictTypeStatusDB 切换字典类型的启用状态，返回更新后的完整行。
func SetDictTypeStatusDB(id string, status bool) (*DictType, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	if err := db.Model(&DictType{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return nil, err
	}
	var row DictType
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// FindActiveDictDataByID 按 ID 加载未删除的字典数据项；不存在返回 ErrDictDataNotFound。
func FindActiveDictDataByID(id string) (*DictData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var row DictData
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDictDataNotFound
		}
		return nil, err
	}
	return &row, nil
}

// SetDictDataStatusDB 切换字典数据项的启用状态，返回更新后的完整行。
func SetDictDataStatusDB(id string, status bool) (*DictData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	if err := db.Model(&DictData{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return nil, err
	}
	var row DictData
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListByTypeCode 返回某类型下全部未删除的数据项（树/管理用），按排序与 ID 升序。
func ListByTypeCode(code string) ([]DictData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rows []DictData
	if err := db.Where("type_code = ?", code).Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListEnabledByTypeCode 返回某类型下启用状态的数据项（下拉用），按排序与 ID 升序。
func ListEnabledByTypeCode(code string) ([]DictData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rows []DictData
	if err := db.Where("type_code = ? AND status = ?", code, true).Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
