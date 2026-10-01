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

// 默认种子：每个 SDK 类型含一个 repo 来源（兼容旧 D:\opt\xxx 布局）。
var defaultSdkSeeds = []struct {
	Name string
	Root string
}{
	{"jdk", `D:\opt\jdk`},
	{"maven", `D:\opt\maven`},
	{"python", `D:\opt\python`},
	{"llama", `D:\opt\llama`},
}

// parseSources 解析 Sources JSON 数组。
func parseSources(raw json.RawMessage) []SdkSourceItem {
	var items []SdkSourceItem
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &items)
	}
	return items
}
