package sdk

import (
	"encoding/json"
	"errors"

	"gorm.io/plugin/soft_delete"
)

const (
	linkName    = "current"
	versionFile = ".current-version"
)

var ErrDBUnavailable = errors.New("sdk: database unavailable")

var (
	ErrSdkNotFound = errors.New("sdk: SDK type not found")

	ErrNameRequired = errors.New("sdk: Name is required")

	ErrInvalidInput = errors.New("sdk: 入参不合法")
)

type InvalidInputError struct{ Msg string }

func (e *InvalidInputError) Error() string { return e.Msg }

func (e *InvalidInputError) Is(target error) bool { return target == ErrInvalidInput }

type SourceExistsError struct{ Name string }

func (e *SourceExistsError) Error() string { return "SDK type already exists: " + e.Name }

func (e *SourceExistsError) Is(target error) bool { return target == ErrSourceExists }

var ErrSourceExists = errors.New("sdk: SDK type already exists")

type SdkSourceItem struct {
	Kind string
	Root string
}

type SdkSource struct {
	ID        uint            `gorm:"primaryKey"`
	Name      string          `gorm:"uniqueIndex:idx_sdk_sources_name,priority:1"`
	Sources   json.RawMessage `gorm:"type:jsonb"`
	Current   string
	Enabled   bool `gorm:"default:true"`
	Note      string
	IsDeleted soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

type SdkInfo struct {
	CurrentVersion string
	VersionList    []string
}

func parseSources(raw json.RawMessage) []SdkSourceItem {
	var items []SdkSourceItem
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &items)
	}
	return items
}
