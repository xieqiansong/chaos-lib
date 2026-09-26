package cronjob

import (
	"time"

	"chaos-go/internal/crud"
)

// ActionType 是定时任务到点后要执行的动作类型。
type ActionType string

const (
	// ActionTypeHTTP 调用 HTTP 接口 / Webhook。
	ActionTypeHTTP ActionType = "http"
	// ActionTypeShell 执行命令 / 脚本（受 FEATURE_CRON_SHELL 开关控制，默认关闭）。
	ActionTypeShell ActionType = "shell"
)

// CronJob 是独立的定时任务（cron job），与任务计划 / 待办任务完全无关。
// 嵌入 crud.BaseModel 自动获得 ID / CreatedAt / UpdatedAt / IsDeleted。
type CronJob struct {
	crud.BaseModel
	Name         string     `json:"Name"`
	CronExpr     string     `json:"CronExpr"`
	ActionType   ActionType `json:"ActionType"`
	ActionConfig string     `json:"ActionConfig"`
	Enabled      bool       `json:"Enabled"`
	TimeoutSec   int        `json:"TimeoutSec"`
	LastRunAt    *time.Time `json:"LastRunAt"`
	LastStatus   string     `json:"LastStatus"` // ok / failed / ""
}

// TableName 显式指定表名（与前端资源名 cronJob 对应）。
func (CronJob) TableName() string { return "cron_jobs" }

// CronJobView 是本资源的响应形态：模型 + 派生字段。
// NextRun（下次执行时间）由后端按 cron 表达式实时计算，不落库。
type CronJobView struct {
	CronJob
	NextRun *time.Time `json:"NextRun"`
}

// CronJobRun 是单次执行的运行日志（只追加、不软删，故不套 CRUD 基字段）。
type CronJobRun struct {
	ID         int `gorm:"primaryKey" json:"ID"`
	JobID      int `json:"JobID"`
	StartedAt  time.Time  `json:"StartedAt"`
	FinishedAt *time.Time `json:"FinishedAt"`
	Success    bool       `json:"Success"`
	Output     string     `gorm:"type:text" json:"Output"`
	Error      string     `gorm:"type:text" json:"Error"`
	CreatedAt  time.Time  `json:"CreatedAt"`
}

// HTTPAction 是 http 类型动作的配置（存于 CronJob.ActionConfig 的 JSON）。
type HTTPAction struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// ShellAction 是 shell 类型动作的配置（默认禁用，仅 FEATURE_CRON_SHELL=true 时真正执行）。
type ShellAction struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"workDir,omitempty"`
}
