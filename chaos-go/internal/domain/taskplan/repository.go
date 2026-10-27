package taskplan

import (
	"errors"
	"time"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/pagination"
	"gorm.io/gorm"
)

var ErrDBUnavailable = errors.New("taskplan: database unavailable")

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

func CreateTaskPlanRow(plan *TaskPlan) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(plan).Error
}

func UpdateTaskPlanColumns(id int, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&TaskPlan{}).Where("id = ?", id).Updates(updates).Error
}

func SaveTaskPlan(plan *TaskPlan) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Save(plan).Error
}

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

func UpdatePlanColumnsByIDs(ids []int, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&TaskPlan{}).Where("id IN ?", ids).Updates(updates).Error
}

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

func SaveTask(task *Task) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Save(task).Error
}

func UpdateTaskColumns(id int, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&Task{}).Where("id = ?", id).Updates(updates).Error
}

func CreateTask(task *Task) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(task).Error
}

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

func QueryPendingTasks(now time.Time, early bool, planID int, name, sort, order string, q pagination.Query) ([]PendingTask, int64, error) {
	db := config.GetDB()
	if db == nil {
		return nil, 0, ErrDBUnavailable
	}
	base := db.Table("tasks").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ?", TaskStatusActive).
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

type ContributionRow struct {
	PlanID    int
	StartedAt time.Time
}

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
