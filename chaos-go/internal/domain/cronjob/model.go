package cronjob

import (
	"errors"
	"time"

	"chaos-go/internal/framework/crud"
)

var ErrJobNotFound = errors.New("cronjob: 定时任务不存在")

var ErrDBUnavailable = errors.New("cronjob: database unavailable")

type ActionType string

const (
	ActionTypeHTTP ActionType = "http"

	ActionTypeShell ActionType = "shell"
)

type CronJob struct {
	crud.BaseModel
	Name         string     `json:"Name"`
	CronExpr     string     `json:"CronExpr"`
	ActionType   ActionType `json:"ActionType"`
	ActionConfig string     `json:"ActionConfig"`
	Enabled      bool       `json:"Enabled"`
	TimeoutSec   int        `json:"TimeoutSec"`
	LastRunAt    *time.Time `json:"LastRunAt"`
	LastStatus   string     `json:"LastStatus"`
}

func (CronJob) TableName() string { return "cron_jobs" }

type CronJobRun struct {
	ID         int        `gorm:"primaryKey" json:"ID"`
	JobID      int        `json:"JobID"`
	StartedAt  time.Time  `json:"StartedAt"`
	FinishedAt *time.Time `json:"FinishedAt"`
	Success    bool       `json:"Success"`
	Output     string     `gorm:"type:text" json:"Output"`
	Error      string     `gorm:"type:text" json:"Error"`
	CreatedAt  time.Time  `json:"CreatedAt"`
}

type HTTPAction struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type ShellAction struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"workDir,omitempty"`
}
