package sdk

import (
	"encoding/json"
	"errors"
)

const (
	linkName    = "current"
	versionFile = ".current-version"
)

// ErrDBUnavailable 表示数据库单例不可用。
var ErrDBUnavailable = errors.New("sdk: database unavailable")

// 领域错误哨兵：调用方据此映射 HTTP 状态码。
var (
	// ErrSdkNotFound 指定 SDK 类型不存在（或已软删）。
	ErrSdkNotFound = errors.New("sdk: SDK type not found")
	// ErrNameRequired 创建时缺少类型名。
	ErrNameRequired = errors.New("sdk: Name is required")
	// ErrInvalidInput 入参不合法（来源 kind / root 等），错误文本可直接呈现给用户。
	ErrInvalidInput = errors.New("sdk: 入参不合法")
)

// InvalidInputError 入参不合法：Msg 面向用户可直接呈现，
// 同时 errors.Is(err, ErrInvalidInput) 为 true，便于调用方统一映射 400。
type InvalidInputError struct{ Msg string }

func (e *InvalidInputError) Error() string { return e.Msg }

func (e *InvalidInputError) Is(target error) bool { return target == ErrInvalidInput }

// SourceExistsError 同名 SDK 类型已存在（映射 409）。
type SourceExistsError struct{ Name string }

func (e *SourceExistsError) Error() string { return "SDK type already exists: " + e.Name }

func (e *SourceExistsError) Is(target error) bool { return target == ErrSourceExists }

// ErrSourceExists 同名冲突的判定哨兵。
var ErrSourceExists = errors.New("sdk: SDK type already exists")

// SdkSourceItem 是 Sources JSON 数组中的一个来源元素。
type SdkSourceItem struct {
	Kind string // "repo" | "single"
	Root string // 绝对路径
}

// SdkSource 一行代表一个 SDK 类型（jdk/maven/python/...）。
// Sources 为该类型下的来源数组（repo 与 single 可混合）；
// Current 为当前启用版本的绝对路径（单值即保证同时仅一个版本启用）。
type SdkSource struct {
	ID        uint            `gorm:"primaryKey"`
	Name      string          `gorm:"uniqueIndex:idx_sdk_sources_name,priority:1"` // SDK 类型，如 jdk / maven
	Sources   json.RawMessage `gorm:"type:jsonb"`                                  // [{"kind":"repo","root":"..."},{"kind":"single","root":"..."}]
	Current   string          // 当前启用版本绝对路径
	Enabled   bool            `gorm:"default:true"`
	Note      string
	IsDeleted bool `gorm:"default:false"`
}

// SdkInfo 返回给前端的版本信息。
type SdkInfo struct {
	CurrentVersion string
	VersionList    []string
}

// parseSources 解析 Sources JSON 数组。
func parseSources(raw json.RawMessage) []SdkSourceItem {
	var items []SdkSourceItem
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &items)
	}
	return items
}
