package services

import (
	"chaos-go/config"
	"chaos-go/models"
	"chaos-go/tasks"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	schedulerInterval = time.Minute // 后台扫描周期
	cronLookahead     = 3           // 每个 cron 计划至少预排的未来任务数（同时作为预排窗口上限，防止高频 cron 爆量）
	cronSweepCap      = 50          // 单次扫描每个计划最多生成的任务数（安全阀）
)

// init 将定时扫描注册到现有的后台任务框架。
// main.go 中已调用 tasks.Start()，无需修改启动流程。
func init() {
	tasks.Register("scheduled-task-scheduler", schedulerInterval, SweepScheduledTaskPlans, nil)
}

// SweepScheduledTaskPlans 为 started 的周期计划补齐任务实例：
//   - cron 计划：补齐未来若干条（解决「不手动完成就不出现下一条」与「一天多次 cron 只生成一条」）
//   - interval 计划：若没有任何 active 任务则补一条，防止计划卡死
func SweepScheduledTaskPlans() {
	sweepCronPlans()
	sweepIntervalPlans()
}

func sweepCronPlans() {
	db := config.GetDB()
	var plans []models.TaskPlan
	if err := db.Where("plan_type = ? AND status = ? AND is_deleted = ?",
		models.TaskPlanTypeCron, models.TaskPlanStatusStarted, false).
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
		db.Model(&models.Task{}).
			Where("plan_id = ? AND status = ? AND started_at > ? AND is_deleted = ?",
				plan.ID, models.TaskStatusActive, now, false).
			Count(&upcoming)
		if upcoming >= cronLookahead {
			continue
		}

		// 以该计划已有任务中最晚的 started_at 为基准，避免每次都从 now 重头生成
		base := now
		var last models.Task
		if err := db.Where("plan_id = ? AND is_deleted = ?", plan.ID, false).
			Order("started_at DESC").First(&last).Error; err == nil && last.StartedAt != nil {
			base = *last.StartedAt
		}

		generated := 0
		t := base
		for int(upcoming)+generated < cronLookahead && generated < cronSweepCap {
			next := schedule.Next(t)
			if !next.After(now) { // 安全阀：永远向前推进
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

// sweepIntervalPlans 兜底：若 started 的 interval 计划没有任何 active 任务，
// 说明计划卡死（异常未续期），补一条立即到期的任务。
func sweepIntervalPlans() {
	db := config.GetDB()
	var plans []models.TaskPlan
	if err := db.Where("plan_type = ? AND status = ? AND is_deleted = ?",
		models.TaskPlanTypeInterval, models.TaskPlanStatusStarted, false).
		Find(&plans).Error; err != nil {
		slog.Error("间隔任务扫描失败", "err", err)
		return
	}

	now := time.Now()
	for i := range plans {
		plan := plans[i]
		var active int64
		db.Model(&models.Task{}).
			Where("plan_id = ? AND status = ? AND is_deleted = ?",
				plan.ID, models.TaskStatusActive, false).
			Count(&active)
		if active > 0 {
			continue
		}
		if _, err := generateTask(&plan, now, nil); err != nil {
			slog.Warn("补充间隔任务失败", "planId", plan.ID, "err", err)
		}
	}
}

// createCronTask 在指定时刻为 cron 计划创建一条任务；
// 若同一时刻的任务已存在则直接返回（幂等，避免重复生成）。
func createCronTask(plan *models.TaskPlan, startedAt time.Time) (*models.Task, error) {
	if taskExistsAt(plan.ID, startedAt) {
		var existing models.Task
		if err := config.GetDB().Where("plan_id = ? AND started_at = ? AND is_deleted = ?",
			plan.ID, startedAt, false).First(&existing).Error; err == nil {
			return &existing, nil
		}
	}
	endOfDay := time.Date(startedAt.Year(), startedAt.Month(), startedAt.Day(),
		23, 59, 59, 0, startedAt.Location())
	task := models.Task{
		PlanID:    plan.ID,
		Status:    models.TaskStatusActive,
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
	config.GetDB().Model(&models.Task{}).
		Where("plan_id = ? AND started_at = ? AND is_deleted = ?", planID, t, false).
		Count(&c)
	return c > 0
}
