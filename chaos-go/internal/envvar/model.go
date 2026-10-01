// Package envvar 环境变量管理：把系统环境变量（Windows 注册表 system/user 作用域）
// 暴露为 quickedit 虚拟文件，支持读取 / 同步 / 局部改写 / 整体替换，并落快照以便回滚。
//
// 双后端：Windows 走注册表（registry_windows.go），其它平台为只读桩（registry_other.go）。
// 模型类型集中在本文件；TOML 序列化、JSON 辅助与字符串工具在 service 层（纯函数，无 IO）。
package envvar

type EnvScope string

const (
	EnvScopeSystem EnvScope = "system"
	EnvScopeUser   EnvScope = "user"
)

type EnvMeta struct {
	SavedAt  string `toml:"saved_at"`
	Hostname string `toml:"hostname,omitempty"`
	Username string `toml:"username,omitempty"`
}

type EnvSection map[string]string

type EnvSnapshot struct {
	Meta   EnvMeta    `toml:"meta"`
	System EnvSection `toml:"system"`
	User   EnvSection `toml:"user"`
}

type EnvPathPatch struct {
	Prepend []string
	Append  []string
	Remove  []string
	Replace []string
}

type EnvSectionPatch struct {
	Set   map[string]string
	Unset []string
	Path  EnvPathPatch
}

type EnvPatchRequest struct {
	System *EnvSectionPatch
	User   *EnvSectionPatch
}

type EnvPutRequest struct {
	System *EnvSection
	User   *EnvSection
}

type EnvGetResponse struct {
	Meta         EnvMeta
	System       EnvSection
	User         EnvSection
	SnapshotID   int
	SnapshotTime string
	Warnings     []string
}

type EnvApplyResponse struct {
	Message      string
	SnapshotID   int
	SnapshotTime string
	Warnings     []string
}
