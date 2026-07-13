package models

import "time"

type TaskPlanStatus string

const (
	TaskPlanStatusCreated   TaskPlanStatus = "created"
	TaskPlanStatusStarted   TaskPlanStatus = "started"
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
	ID                int            `gorm:"primaryKey"`
	ParentID          *int           ``
	Name              string         ``
	Status            TaskPlanStatus `gorm:"default:created"`
	PlanType          TaskPlanType   `gorm:"default:todo"`
	CronExpr          *string        ``
	OrderNum          int            `gorm:"default:0"`
	Priority          int            `gorm:"default:5"`
	FsrsStability     float64        `gorm:"default:0"`
	FsrsDifficulty    float64        `gorm:"default:0"`
	FsrsReps          int            `gorm:"default:0"`
	FsrsLapses        int            `gorm:"default:0"`
	FsrsState         int            `gorm:"default:0"`
	FsrsLearningSteps int            `gorm:"default:0"`
	FsrsLastReviewAt  *time.Time     ``
	ContentSize       int            `gorm:"default:0"`
	Remark            *string        ``
	Link              *string        ``
	CreatedAt         time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt         time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	IsDeleted         bool           `gorm:"default:false"`
	IsSuspended       bool           `gorm:"default:false"`
}

func (TaskPlan) TableName() string {
	return "task_plans"
}

type TaskPlanTree struct {
	TaskPlan
	Children []TaskPlanTree ``
}
