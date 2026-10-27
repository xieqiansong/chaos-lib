package taskplan

import (
	"errors"
	"time"

	"gorm.io/plugin/soft_delete"
)

var (
	ErrPlanNotFound = errors.New("taskplan: 任务计划不存在")
	ErrTaskNotFound = errors.New("taskplan: 任务不存在")
	ErrNoRawLink    = errors.New("taskplan: 该计划没有 raw_link，无法获取原文")

	ErrInvalidState = errors.New("taskplan: 业务前置条件不满足")
)

type InvalidStateError struct{ Msg string }

func (e *InvalidStateError) Error() string { return e.Msg }

func (e *InvalidStateError) Is(target error) bool { return target == ErrInvalidState }

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID            int                   `gorm:"primaryKey"`
	PlanID        int                   ``
	Status        TaskStatus            `gorm:"default:pending"`
	ScheduledDate *time.Time            ``
	StartedAt     *time.Time            ``
	CompletedAt   *time.Time            ``
	Deadline      *time.Time            ``
	Rating        *int                  ``
	Remark        *string               ``
	IsDeleted     soft_delete.DeletedAt `gorm:"softDelete:flag"`
	CreatedAt     time.Time             `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time             `gorm:"default:CURRENT_TIMESTAMP"`
}

type PendingTask struct {
	Task
	PlanName    string         ``
	PlanStatus  TaskPlanStatus ``
	PlanType    TaskPlanType   ``
	Link        *string        ``
	RawLink     *string        ``
	ContentSize int            ``
	FsrsReps    int            ``
	IsOverdue   bool           ``
}

type TaskPlanStatus string

const (
	TaskPlanStatusCreated   TaskPlanStatus = "created"
	TaskPlanStatusStarted   TaskPlanStatus = "started"
	TaskPlanStatusSuspended TaskPlanStatus = "suspended"
	TaskPlanStatusCompleted TaskPlanStatus = "completed"
	TaskPlanStatusArchived  TaskPlanStatus = "archived"
)

type TaskPlanType string

const (
	TaskPlanTypeTodo     TaskPlanType = "todo"
	TaskPlanTypeCron     TaskPlanType = "cron"
	TaskPlanTypeInterval TaskPlanType = "interval"
)

type TaskPlan struct {
	ID                int                   `gorm:"primaryKey"`
	ParentID          *int                  ``
	Name              string                ``
	Code              *string               ``
	Status            TaskPlanStatus        `gorm:"default:created"`
	PlanType          TaskPlanType          `gorm:"default:todo"`
	Priority          int                   `gorm:"default:5"`
	OrderNum          int                   `gorm:"default:0"`
	Link              *string               ``
	RawLink           *string               ``
	Remark            *string               ``
	ContentSize       *int                  ``
	CronExpr          *string               ``
	IntervalDays      *int                  ``
	IntervalHour      *int                  ``
	IntervalMinute    *int                  ``
	TaskCount         int                   `gorm:"default:0"`
	CompletedCount    int                   `gorm:"default:0"`
	IsSuspended       bool                  `gorm:"default:false"`
	TotalStudyTime    *int                  `gorm:"default:0"`
	LastCompletedAt   *time.Time            ``
	FsrsStability     float64               `gorm:"default:0"`
	FsrsDifficulty    float64               `gorm:"default:0"`
	FsrsReps          int                   `gorm:"default:0"`
	FsrsLapses        int                   `gorm:"default:0"`
	FsrsState         int                   `gorm:"default:0"`
	FsrsLearningSteps int                   `gorm:"default:0"`
	FsrsLastReviewAt  *time.Time            ``
	IsDeleted         soft_delete.DeletedAt `gorm:"softDelete:flag"`
	CreatedAt         time.Time             `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt         time.Time             `gorm:"default:CURRENT_TIMESTAMP"`
}

type TaskPlanTree struct {
	TaskPlan
	Children []TaskPlanTree ``
	HasLink  bool           ``
}
