package cronjob

import "time"

// ActionType 是定时任务到点后要执行的动作类型。
type ActionType string

const (
	// ActionTypeHTTP 调用 HTTP 接口 / Webhook。
	ActionTypeHTTP ActionType = "http"
	// ActionTypeShell 执行命令 / 脚本（预留，受 FEATURE_CRON_SHELL 开关控制，默认关闭）。
	ActionTypeShell ActionType = "shell"
)

// CronJob 是独立的定时任务（cron job），与任务计划 / 待办任务完全无关。
type CronJob struct {
	ID           int        `gorm:"primaryKey"`
	Name         string     `gorm:"size:255;not null"`
	CronExpr     string     `gorm:"size:255;not null"`
	ActionType   ActionType `gorm:"size:32;default:http"`
	ActionConfig string     `gorm:"type:text"`
	Enabled      bool       `gorm:"default:true"`
	TimeoutSec   int        `gorm:"default:30"`
	LastRunAt    *time.Time
	LastStatus   string     `gorm:"size:16;default:''"` // ok / failed / ""
	IsDeleted    bool       `gorm:"default:false"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CronJobRun 是单次执行的运行日志。
type CronJobRun struct {
	ID         int `gorm:"primaryKey"`
	JobID      int
	StartedAt  time.Time
	FinishedAt *time.Time
	Success    bool
	Output     string `gorm:"type:text"`
	Error      string `gorm:"type:text"`
	CreatedAt  time.Time
}

// HTTPAction 是 http 类型动作的配置（存于 CronJob.ActionConfig 的 JSON）。
type HTTPAction struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// ShellAction 是 shell 类型动作的配置（预留，默认禁用）。
type ShellAction struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"workDir,omitempty"`
}
