package taskplan

import (
	"time"
)

// TaskStatus 任务执行状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID            int        `gorm:"primaryKey"`
	PlanID        int        ``
	Status        TaskStatus `gorm:"default:pending"`
	ScheduledDate *time.Time ``
	StartedAt     *time.Time ``
	CompletedAt   *time.Time ``
	Deadline      *time.Time ``
	Rating        *int       ``
	Remark        *string    ``
	IsDeleted     bool       `gorm:"default:false"`
	CreatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
}

// PendingTask 待办任务视图（联表查询）
type PendingTask struct {
	Task
	PlanName    string         ``
	PlanStatus  TaskPlanStatus ``
	PlanType    TaskPlanType   ``
	Link        *string        ``
	ContentSize int            ``
	FsrsReps    int            ``
	IsOverdue   bool           ``
}
