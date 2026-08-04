package taskplan

import (
	"chaos-go/config"
	"chaos-go/scheduler"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	schedulerInterval = time.Minute
	cronLookahead     = 3
	cronSweepCap      = 50
)

func init() {
	scheduler.Register("scheduled-task-scheduler", schedulerInterval, SweepScheduledTaskPlans)
}

func SweepScheduledTaskPlans() {
	sweepCronPlans()
	sweepIntervalPlans()
}

func sweepCronPlans() {
	db := config.GetDB()
	var plans []TaskPlan
	if err := db.Where("plan_type = ? AND status = ? AND is_deleted = ?",
		TaskPlanTypeCron, TaskPlanStatusStarted, false).
		Find(&plans).Error; err != nil {
		slog.Error("周期任务扫描失败", "err", err)
		return
	}
	now := time.Now()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for i := range plans {
		plan := plans[i]
		if plan.CronExpr == nil || *plan.CronExpr == "" {
			continue
		}
		schedule, err := parser.Parse(*plan.CronExpr)
		if err != nil {
			slog.Warn("计划的 cron 表达式无效", "planId", plan.ID, "err", err)
			continue
		}
		var upcoming int64
		db.Model(&Task{}).
			Where("plan_id = ? AND status = ? AND started_at > ? AND is_deleted = ?",
				plan.ID, TaskStatusActive, now, false).
			Count(&upcoming)
		if upcoming >= cronLookahead {
			continue
		}
		base := now
		var last Task
		if err := db.Where("plan_id = ? AND is_deleted = ?", plan.ID, false).
			Order("started_at DESC").First(&last).Error; err == nil && last.StartedAt != nil {
			base = *last.StartedAt
		}
		generated := 0
		t := base
		for int(upcoming)+generated < cronLookahead && generated < cronSweepCap {
			next := schedule.Next(t)
			if !next.After(now) {
				t = next
				continue
			}
			if _, err := createCronTask(&plan, next); err != nil {
				slog.Warn("生成 cron 任务失败", "planId", plan.ID, "err", err)
				break
			}
			generated++
			t = next
		}
	}
}

func sweepIntervalPlans() {
	db := config.GetDB()
	var plans []TaskPlan
	if err := db.Where("plan_type = ? AND status = ? AND is_deleted = ?",
		TaskPlanTypeInterval, TaskPlanStatusStarted, false).
		Find(&plans).Error; err != nil {
		slog.Error("间隔任务扫描失败", "err", err)
		return
	}
	now := time.Now()
	for i := range plans {
		plan := plans[i]
		var active int64
		db.Model(&Task{}).
			Where("plan_id = ? AND status = ? AND is_deleted = ?",
				plan.ID, TaskStatusActive, false).
			Count(&active)
		if active > 0 {
			continue
		}
		if _, err := generateTask(&plan, now, nil); err != nil {
			slog.Warn("补充间隔任务失败", "planId", plan.ID, "err", err)
		}
	}
}

func createCronTask(plan *TaskPlan, startedAt time.Time) (*Task, error) {
	if taskExistsAt(plan.ID, startedAt) {
		var existing Task
		if err := config.GetDB().Where("plan_id = ? AND started_at = ? AND is_deleted = ?",
			plan.ID, startedAt, false).First(&existing).Error; err == nil {
			return &existing, nil
		}
	}
	endOfDay := time.Date(startedAt.Year(), startedAt.Month(), startedAt.Day(),
		23, 59, 59, 0, startedAt.Location())
	task := Task{
		PlanID:    plan.ID,
		Status:    TaskStatusActive,
		StartedAt: &startedAt,
		Deadline:  &endOfDay,
	}
	if err := config.GetDB().Create(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func taskExistsAt(planID int, t time.Time) bool {
	var c int64
	config.GetDB().Model(&Task{}).
		Where("plan_id = ? AND started_at = ? AND is_deleted = ?", planID, t, false).
		Count(&c)
	return c > 0
}
