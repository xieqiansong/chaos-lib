package models

import "time"

type TaskStatus string

const (
	TaskStatusActive    TaskStatus = "active"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID          int        `gorm:"primaryKey"`
	PlanID      int        ``
	Status      TaskStatus `gorm:"default:active"`
	StartedAt   *time.Time ``
	CompletedAt *time.Time ``
	Deadline    *time.Time ``
	Remark      *string    ``
	CreatedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	IsDeleted   bool       `gorm:"default:false"`
}

func (Task) TableName() string {
	return "tasks"
}

type PendingTask struct {
	Task
	PlanName    string       ``
	PlanType    TaskPlanType ``
	Link        *string      ``
	ContentSize int          ``
	IsOverdue   bool         ``
}
