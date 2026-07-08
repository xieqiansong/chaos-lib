package models

type EnvScope string

const (
	EnvScopeSystem EnvScope = "system"
	EnvScopeUser   EnvScope = "user"
)

type EnvMeta struct {
	SavedAt  string `toml:"saved_at" `
	Hostname string `toml:"hostname,omitempty" `
	Username string `toml:"username,omitempty" `
}

type EnvSection map[string]string

type EnvSnapshot struct {
	Meta   EnvMeta    `toml:"meta" `
	System EnvSection `toml:"system" `
	User   EnvSection `toml:"user" `
}

type EnvPathPatch struct {
	Prepend []string ``
	Append  []string ``
	Remove  []string ``
	Replace []string ``
}

type EnvSectionPatch struct {
	Set   map[string]string ``
	Unset []string          ``
	Path  EnvPathPatch      ``
}

type EnvPatchRequest struct {
	System *EnvSectionPatch ``
	User   *EnvSectionPatch ``
}

type EnvPutRequest struct {
	System *EnvSection ``
	User   *EnvSection ``
}

type EnvGetResponse struct {
	Meta         EnvMeta    ``
	System       EnvSection ``
	User         EnvSection ``
	SnapshotID   int        ``
	SnapshotTime string     ``
	Warnings     []string   ``
}

type EnvApplyResponse struct {
	Message      string   ``
	SnapshotID   int      ``
	SnapshotTime string   ``
	Warnings     []string ``
}

const (
	EnvVirtualFilePath = "__env__all__"
	EnvVirtualFileName = "环境变量"
	EnvVirtualRemark   = "系统 + 用户环境变量快照（TOML 格式）"
)
