package taskplan

import (
	"errors"
	"time"

	"chaos-go/internal/config"
	"chaos-go/internal/pagination"
	"gorm.io/gorm"
)

// ErrDBUnavailable 表示数据库单例不可用。
var ErrDBUnavailable = errors.New("taskplan: database unavailable")

// ── 计划树 / 子孙归集 ──

// CollectDescendantPlanIDs 收集 rootID 及其所有子孙 ID（含自身）。
func CollectDescendantPlanIDs(rootID int) ([]int, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	ids := []int{}
	var walk func(int) error
	walk = func(id int) error {
		ids = append(ids, id)
		var children []TaskPlan
		if err := db.Where("parent_id = ?", id).Find(&children).Error; err != nil {
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

// ── 计划查询 ──

// FindActiveTaskPlan 按 ID 加载未删除的计划；不存在返回 ErrPlanNotFound。
func FindActiveTaskPlan(id int) (*TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var plan TaskPlan
	if err := db.Where("id = ?", id).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// GetTaskPlanByID 按 ID 加载计划（忽略逻辑删除，用于更新后回读）；不存在返回 ErrPlanNotFound。
func GetTaskPlanByID(id int) (*TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var plan TaskPlan
	if err := db.First(&plan, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// CreateTaskPlanRow 新增计划。
func CreateTaskPlanRow(plan *TaskPlan) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(plan).Error
}

// UpdateTaskPlanColumns 按字段映射更新计划（限定未删除）。
func UpdateTaskPlanColumns(id int, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&TaskPlan{}).Where("id = ?", id).Updates(updates).Error
}

// SaveTaskPlan 全量保存计划（用于状态机迁移后回写）。
func SaveTaskPlan(plan *TaskPlan) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Save(plan).Error
}

// CountChildPlans 统计某计划的直属子计划数（未删除）。
func CountChildPlans(id int) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var c int64
	if err := db.Model(&TaskPlan{}).Where("parent_id = ?", id).Count(&c).Error; err != nil {
		return 0, err
	}
	return c, nil
}

// CountActiveTasks 统计某计划的进行中任务数。
func CountActiveTasks(planID int) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var c int64
	if err := db.Model(&Task{}).Where("plan_id = ? AND status = ?", planID, TaskStatusActive).Count(&c).Error; err != nil {
		return 0, err
	}
	return c, nil
}

// UpdatePlanColumnsByIDs 批量按字段映射更新计划（用于挂起/恢复/优先级）。
func UpdatePlanColumnsByIDs(ids []int, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&TaskPlan{}).Where("id IN ?", ids).Updates(updates).Error
}

// ListPlans 列出计划（按 planType/status 可选过滤，均为未删除）。
func ListPlans(planType, status string) ([]TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	q := db.Model(&TaskPlan{})
	if planType != "" {
		q = q.Where("plan_type = ?", planType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var plans []TaskPlan
	if err := q.Order("order_num ASC, priority DESC, created_at DESC, id DESC").Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

// ListPlanTreeRows 列出构建树所需的计划投影列（未删除）。
func ListPlanTreeRows() ([]TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var plans []TaskPlan
	if err := db.Select("ID", "ParentID", "Name", "Status", "PlanType", "TaskCount", "Priority", "OrderNum", "Link", "IsSuspended", "FsrsReps").
		Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

// ── 任务查询 ──

// FindActiveTask 按 ID 加载未删除的任务；不存在返回 ErrTaskNotFound。
func FindActiveTask(id int) (*Task, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var task Task
	if err := db.Where("id = ?", id).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// FindActiveTaskForPlan 返回某计划进行中的任务（不存在时 ok=false）。
func FindActiveTaskForPlan(planID int) (*Task, bool) {
	db := config.GetDB()
	if db == nil {
		return nil, false
	}
	var task Task
	if err := db.Where("plan_id = ? AND status = ?", planID, TaskStatusActive).First(&task).Error; err != nil {
		return nil, false
	}
	return &task, true
}

// FindTaskAt 返回某计划在特定 started_at 的任务（不存在时 ok=false）。
func FindTaskAt(planID int, t time.Time) (*Task, bool) {
	db := config.GetDB()
	if db == nil {
		return nil, false
	}
	var task Task
	if err := db.Where("plan_id = ? AND started_at = ?", planID, t).First(&task).Error; err != nil {
		return nil, false
	}
	return &task, true
}

// SaveTask 全量保存任务。
func SaveTask(task *Task) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Save(task).Error
}

// UpdateTaskColumns 按字段映射更新任务。
func UpdateTaskColumns(id int, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&Task{}).Where("id = ?", id).Updates(updates).Error
}

// CreateTask 新增任务。
func CreateTask(task *Task) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(task).Error
}

// ListTasksByPlan 列出某计划的全部任务（未删除）。
func ListTasksByPlan(planID int) ([]Task, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var tasks []Task
	if err := db.Where("plan_id = ?", planID).Order("created_at DESC, id DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// ── 事务类操作 ──

// CompleteActiveTasksForPlan 在事务内将某计划进行中的任务标记完成，并将计划置为 completed。
func CompleteActiveTasksForPlan(planID int, now time.Time) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	tx := db.Begin()
	if err := tx.Model(&Task{}).Where("plan_id = ? AND status = ?", planID, TaskStatusActive).
		Updates(map[string]interface{}{"status": TaskStatusDone, "completed_at": now}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&TaskPlan{}).Where("id = ?", planID).
		Updates(map[string]interface{}{"status": TaskPlanStatusCompleted, "updated_at": now}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// SoftDeletePlanAndTasks 软删除计划（及其任务）。cascade 为真时连同全部子孙计划一并删除。
// 返回 cascade 模式下被删除的计划数量。
func SoftDeletePlanAndTasks(planID int, cascade bool) (int, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	now := time.Now()
	if cascade {
		ids, err := CollectDescendantPlanIDs(planID)
		if err != nil {
			return 0, err
		}
		tx := db.Begin()
		if len(ids) > 0 {
			// Delete 在 soft_delete 插件下即软删（UPDATE ... SET is_deleted = 1）
			if err := tx.Where("plan_id IN ?", ids).Delete(&Task{}).Error; err != nil {
				tx.Rollback()
				return 0, err
			}
		}
		if err := tx.Model(&TaskPlan{}).Where("id IN ?", ids).
			Updates(map[string]interface{}{"is_deleted": 1, "updated_at": now}).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		if err := tx.Commit().Error; err != nil {
			return 0, err
		}
		return len(ids), nil
	}
	tx := db.Begin()
	if err := tx.Where("plan_id = ?", planID).Delete(&Task{}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Model(&TaskPlan{}).Where("id = ?", planID).
		Updates(map[string]interface{}{"is_deleted": 1, "updated_at": now}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	return 0, tx.Commit().Error
}

// ── 调度扫描（被 SweepScheduledTaskPlans 调用）──

// ListScheduledPlans 列出指定类型、状态、未挂起且未删除的计划。
func ListScheduledPlans(planType TaskPlanType, status TaskPlanStatus) ([]TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var plans []TaskPlan
	if err := db.Where("plan_type = ? AND status = ? AND is_suspended = ?",
		planType, status, false).Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

// CountActiveUpcomingTasks 统计某计划未来的进行中任务数（started_at > now）。
func CountActiveUpcomingTasks(planID int, now time.Time) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var c int64
	if err := db.Model(&Task{}).
		Where("plan_id = ? AND status = ? AND started_at > ?", planID, TaskStatusActive, now).
		Count(&c).Error; err != nil {
		return 0, err
	}
	return c, nil
}

// FindLatestTaskByPlan 返回某计划最近一次（按 started_at 倒序）任务。
func FindLatestTaskByPlan(planID int) (*Task, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var last Task
	if err := db.Where("plan_id = ?", planID).Order("started_at DESC").First(&last).Error; err != nil {
		return nil, err
	}
	return &last, nil
}

// ── 待办列表（联表查询）──

// QueryPendingTasks 返回进行中任务及其计划信息的分页列表。
// early=true 时不过滤未来任务；planID/name 为可选过滤；sort/order 为白名单排序覆盖。
func QueryPendingTasks(now time.Time, early bool, planID int, name, sort, order string, q pagination.Query) ([]PendingTask, int64, error) {
	db := config.GetDB()
	if db == nil {
		return nil, 0, ErrDBUnavailable
	}
	base := db.Table("tasks").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ?", TaskStatusActive).
		// 联表走 Table（无模型 schema），soft_delete 插件不生效，两张表的软删条件仍须显式书写
		Where("tasks.is_deleted = ?", 0).
		Where("task_plans.is_deleted = ?", 0).
		Where("task_plans.is_suspended = ?", false)

	if !early {
		base = base.Where("(tasks.started_at IS NULL OR tasks.started_at <= ?)", now)
	}
	if planID > 0 {
		ids, err := CollectDescendantPlanIDs(planID)
		if err != nil {
			return nil, 0, err
		}
		base = base.Where("tasks.plan_id IN ?", ids)
	}
	if name != "" {
		base = base.Where("task_plans.name LIKE ?", "%"+name+"%")
	}

	orderClause := "task_plans.priority DESC, tasks.deadline ASC NULLS LAST, tasks.started_at ASC"
	if sort != "" {
		allowed := map[string]string{
			"started_at": "tasks.started_at",
			"deadline":   "tasks.deadline",
			"plan_name":  "task_plans.name",
		}
		if col, ok := allowed[sort]; ok {
			dir := "ASC"
			if order == "desc" {
				dir = "DESC"
			}
			if sort == "deadline" {
				col += " NULLS LAST"
			}
			orderClause = col + " " + dir
		}
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []struct {
		Task
		PlanName    string       ``
		PlanType    TaskPlanType ``
		PlanLink    *string      ``
		PlanRawLink *string      ``
		ContentSize int          ``
		FsrsReps    int          ``
	}
	if err := base.
		Select("tasks.*, task_plans.name AS plan_name, task_plans.plan_type AS plan_type, task_plans.link AS plan_link, task_plans.raw_link AS plan_raw_link, task_plans.content_size, task_plans.fsrs_reps").
		Order(orderClause).
		Scopes(q.Scope).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]PendingTask, 0, len(rows))
	for _, row := range rows {
		result = append(result, PendingTask{
			Task:        row.Task,
			PlanName:    row.PlanName,
			PlanType:    row.PlanType,
			Link:        row.PlanLink,
			RawLink:     row.PlanRawLink,
			ContentSize: row.ContentSize,
			FsrsReps:    row.FsrsReps,
			IsOverdue:   row.Deadline != nil && now.After(*row.Deadline),
		})
	}
	return result, total, nil
}

// ── 统计查询 ──

// DailyCompletionRows 返回 [cutoff, now] 区间内已完成任务的 completed_at 时间。
func DailyCompletionRows(cutoff time.Time) ([]time.Time, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	type Row struct {
		CompletedAt time.Time
	}
	var rows []Row
	if err := db.Table("tasks").
		Select("completed_at").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.completed_at IS NOT NULL AND tasks.completed_at >= ?",
			TaskStatusDone, 0, cutoff).
		Where("task_plans.is_suspended = ?", false).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.CompletedAt)
	}
	return out, nil
}

// ActiveStartRows 返回 [start, end+1天) 区间内进行中任务的 started_at 时间。
func ActiveStartRows(start, end time.Time) ([]time.Time, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	type Row struct {
		StartedAt time.Time
	}
	var rows []Row
	if err := db.Table("tasks").
		Select("started_at").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.started_at IS NOT NULL AND tasks.started_at >= ? AND tasks.started_at < ?",
			TaskStatusActive, 0, start, end.AddDate(0, 0, 1)).
		Where("task_plans.is_suspended = ?", false).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.StartedAt)
	}
	return out, nil
}

// ContributionRow 贡献热力图的单条聚合行。
type ContributionRow struct {
	PlanID    int
	StartedAt time.Time
}

// ContributionRows 返回指定计划集合内、started_at >= start 的已完成任务的 (plan_id, started_at)。
func ContributionRows(planIDs []int, start time.Time) ([]ContributionRow, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rows []ContributionRow
	if err := db.Table("tasks").
		Select("tasks.plan_id, tasks.started_at").
		Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.started_at IS NOT NULL AND tasks.started_at >= ?",
			TaskStatusDone, 0, start).
		Where("tasks.plan_id IN ?", planIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// FindActiveUnsuspendedPlanByID 按 ID 加载未删除且未挂起的计划；不存在返回 ErrPlanNotFound。
func FindActiveUnsuspendedPlanByID(id int) (*TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var plan TaskPlan
	if err := db.Where("id = ? AND is_suspended = ?", id, false).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// FindActiveUnsuspendedPlansByName 按名称加载未删除且未挂起的计划（可多个）。
func FindActiveUnsuspendedPlansByName(name string) ([]TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var plans []TaskPlan
	if err := db.Where("name = ? AND is_suspended = ?", name, false).Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

// ListChildren 列出某计划的直属子计划（未删除且未挂起）。
func ListChildren(planID int) ([]TaskPlan, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var children []TaskPlan
	if err := db.Where("parent_id = ? AND is_suspended = ?", planID, false).
		Order("order_num ASC, id ASC").Find(&children).Error; err != nil {
		return nil, err
	}
	return children, nil
}
