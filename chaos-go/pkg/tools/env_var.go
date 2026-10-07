package tools

// EnvScope 表示环境变量的作用域（持久化级别）。
// Process 仅在当前进程有效；User 对当前用户永久生效；Machine 对系统全局永久生效。
type EnvScope string

const (
	EnvScopeProcess EnvScope = "Process"
	EnvScopeUser    EnvScope = "User"
	EnvScopeMachine EnvScope = "Machine"
)

// EnvVar 描述一个环境变量的键、值与作用域。
type EnvVar struct {
	Name   string   `json:"Name"`   // 变量名
	Value  string   `json:"Value"`  // 变量值；不存在时为 ""
	Scope  EnvScope `json:"Scope"`  // 所属作用域
	Exists bool     `json:"Exists"` // 变量是否存在（GetEnvVar 时有效）
}

// EnvVarList 为一组环境变量的 JSON 包装，便于 PowerShell ConvertTo-Json 输出解析。
type EnvVarList struct {
	Vars []EnvVar `json:"Vars"`
}
