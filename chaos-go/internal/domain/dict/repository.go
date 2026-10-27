package dict

import (
	"errors"

	"chaos-go/internal/framework/config"

	"gorm.io/gorm"
)

var (
	ErrDBUnavailable    = errors.New("dict: database unavailable")
	ErrDictTypeNotFound = errors.New("dict: 字典类型不存在")
	ErrDictDataNotFound = errors.New("dict: 字典数据不存在")
)

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
