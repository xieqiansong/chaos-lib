package taskplan

import (
	"time"
)

// ── 任务（Task）─────────────────────────────────────────────────

// TaskStatus 任务执行状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task 单条任务记录。
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

// PendingTask 待办任务视图（联表查询）。
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

// ── 任务计划（TaskPlan）─────────────────────────────────────────

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

// TaskPlan 任务计划（可形成树，含 FSRS 记忆参数）。
type TaskPlan struct {
	ID                int            `gorm:"primaryKey"`
	ParentID          *int           ``
	Name              string         ``
	Code              *string        ``
	Status            TaskPlanStatus `gorm:"default:created"`
	PlanType          TaskPlanType   `gorm:"default:todo"`
	Priority          int            `gorm:"default:5"`
	OrderNum          int            `gorm:"default:0"`
	Link              *string        ``
	RawLink           *string        ``
	Remark            *string        ``
	ContentSize       *int           ``
	CronExpr          *string        ``
	IntervalDays      *int           ``
	IntervalHour      *int           ``
	IntervalMinute    *int           ``
	TaskCount         int            `gorm:"default:0"`
	CompletedCount    int            `gorm:"default:0"`
	IsSuspended       bool           `gorm:"default:false"`
	TotalStudyTime    *int           `gorm:"default:0"`
	LastCompletedAt   *time.Time     ``
	FsrsStability     float64        `gorm:"default:0"`
	FsrsDifficulty    float64        `gorm:"default:0"`
	FsrsReps          int            `gorm:"default:0"`
	FsrsLapses        int            `gorm:"default:0"`
	FsrsState         int            `gorm:"default:0"`
	FsrsLearningSteps int            `gorm:"default:0"`
	FsrsLastReviewAt  *time.Time     ``
	IsDeleted         bool           `gorm:"default:false"`
	CreatedAt         time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt         time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
}

// TaskPlanTree 任务计划树节点。
type TaskPlanTree struct {
	TaskPlan
	Children []TaskPlanTree ``
	HasLink  bool           ``
}
