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
