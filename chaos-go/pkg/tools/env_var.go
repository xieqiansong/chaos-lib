package tools

type EnvScope string

const (
	EnvScopeProcess EnvScope = "Process"
	EnvScopeUser    EnvScope = "User"
	EnvScopeMachine EnvScope = "Machine"
)

type EnvVar struct {
	Name   string   `json:"Name"`
	Value  string   `json:"Value"`
	Scope  EnvScope `json:"Scope"`
	Exists bool     `json:"Exists"`
}

type EnvVarList struct {
	Vars []EnvVar `json:"Vars"`
}
