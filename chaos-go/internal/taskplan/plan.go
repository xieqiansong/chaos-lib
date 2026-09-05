package taskplan

import (
	"chaos-go/config"
	"time"
)

// ── 常量 ──────────────────────────────────────────────────────────

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

// ── 模型 ──────────────────────────────────────────────────────────

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

// TaskPlanTree 任务计划树节点
type TaskPlanTree struct {
	TaskPlan
	Children []TaskPlanTree ``
	HasLink  bool           ``
}

// collectPlanWithDescendants 收集 rootID 及其所有子孙 ID（含自身）
func collectPlanWithDescendants(rootID int) ([]int, error) {
	db := config.GetDB()
	ids := []int{}
	var walk func(int) error
	walk = func(id int) error {
		ids = append(ids, id)
		var children []TaskPlan
		if err := db.Where("parent_id = ? AND is_deleted = ?", id, false).Find(&children).Error; err != nil {
			return err
		}
		for _, child := range children {
			if err := walk(child.ID); err != nil {
				return err
			}
		}
		return nil
	}
	return ids, walk(rootID)
}
