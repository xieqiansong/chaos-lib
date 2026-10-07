// Package envvar 环境变量管理：把系统环境变量（Windows 注册表 system/user 作用域）
// 暴露为 quickedit 虚拟文件，支持读取 / 同步 / 局部改写 / 整体替换，并落快照以便回滚。
//
// 双后端：Windows 走注册表（registry_windows.go），其它平台为只读桩（registry_other.go）。
//
// 分层（见 chaos-lib/AGENTS.md「分层契约」）：
//   - model.go：实体 + 请求 / 响应契约
//   - repository.go：唯一数据访问出口（虚拟文件与快照，经 quickedit API）
//   - service.go：编解码 + 用例编排
//   - handler.go：参数解析 + 状态码映射 + 响应
//   - util.go：包内私有纯工具
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

// EnvGetRequest GET 请求；Scope 为空时返回 system + user 两段，为 system / user 时仅返回对应段。
type EnvGetRequest struct {
	Scope EnvScope `json:"scope,omitempty"`
}

type EnvGetResponse struct {
	Meta         EnvMeta
	System       EnvSection
	User         EnvSection
	SnapshotID   int
	SnapshotTime string
	Warnings     []string
}

// SnapshotDetail 单条快照详情：解析成功时给出结构化段落，解析失败时退化为原文 + 错误信息。
type SnapshotDetail struct {
	ID         int        `json:"id"`
	FileID     int        `json:"fileId"`
	Meta       *EnvMeta   `json:"meta"`
	System     EnvSection `json:"system"`
	User       EnvSection `json:"user"`
	RawContent string     `json:"rawContent"`
	ParseError string     `json:"parseError,omitempty"`
	CreatedAt  string     `json:"createdAt"`
	SizeBytes  int        `json:"sizeBytes"`
}

type EnvApplyResponse struct {
	Message      string
	SnapshotID   int
	SnapshotTime string
	Warnings     []string
}
