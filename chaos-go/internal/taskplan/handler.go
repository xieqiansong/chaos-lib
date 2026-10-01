package taskplan

import (
	"chaos-go/internal/config"
	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

var fsrsInstance = NewFsrs(nil)

func getPlanID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

func getTaskID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

func buildTaskPlanTree(plans []TaskPlan) []TaskPlanTree {
	childrenMap := make(map[int][]TaskPlan)
	var roots []TaskPlan

	for _, p := range plans {
		if p.ParentID == nil {
			roots = append(roots, p)
		} else {
			childrenMap[*p.ParentID] = append(childrenMap[*p.ParentID], p)
		}
	}

	sortByOrder := func(list []TaskPlan) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].OrderNum != list[j].OrderNum {
				return list[i].OrderNum < list[j].OrderNum
			}
			return list[i].ID < list[j].ID
		})
	}

	sortByOrder(roots)
	for _, children := range childrenMap {
		sortByOrder(children)
	}

	var build func(parent TaskPlan) TaskPlanTree
	build = func(parent TaskPlan) TaskPlanTree {
		node := TaskPlanTree{TaskPlan: parent}
		node.HasLink = parent.Link != nil
		node.Link = nil
		for _, child := range childrenMap[parent.ID] {
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	var result []TaskPlanTree
	for _, root := range roots {
		result = append(result, build(root))
	}
	return result
}

func generateTask(plan *TaskPlan, now time.Time, rating *FsrsRating) (*Task, error) {
	now = now.Truncate(time.Second)
	task := Task{
		PlanID:    plan.ID,
		Status:    TaskStatusActive,
		StartedAt: &now,
	}

	switch plan.PlanType {
	case TaskPlanTypeCron:
		if plan.CronExpr == nil || *plan.CronExpr == "" {
			return nil, fmt.Errorf("cron表达式不能为空")
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(*plan.CronExpr)
		if err != nil {
			return nil, fmt.Errorf("无效的cron表达式: %v", err)
		}
		return createCronTask(plan, schedule.Next(now))

	case TaskPlanTypeInterval:
		if rating != nil {
			fsrsCard := FsrsCard{
				Stability:     plan.FsrsStability,
				Difficulty:    plan.FsrsDifficulty,
				Reps:          plan.FsrsReps,
				Lapses:        plan.FsrsLapses,
				State:         FsrsState(plan.FsrsState),
				LearningSteps: plan.FsrsLearningSteps,
			}
			if plan.FsrsLastReviewAt != nil {
				fsrsCard.LastReview = plan.FsrsLastReviewAt
			}
			result := fsrsInstance.Next(&fsrsCard, now, *rating)
			task.StartedAt = &result.Due

			plan.FsrsStability = result.Card.Stability
			plan.FsrsDifficulty = result.Card.Difficulty
			plan.FsrsReps = result.Card.Reps
			plan.FsrsLapses = result.Card.Lapses
			plan.FsrsState = int(result.Card.State)
			plan.FsrsLearningSteps = result.Card.LearningSteps
			plan.FsrsLastReviewAt = &now
			plan.UpdatedAt = now
		}
	}

	if err := config.GetDB().Create(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func buildTaskResponse(task Task) gin.H {
	resp := gin.H{
		"id":        task.ID,
		"planId":    task.PlanID,
		"status":    task.Status,
		"createdAt": task.CreatedAt,
	}
	if task.StartedAt != nil {
		resp["startedAt"] = task.StartedAt
	}
	if task.CompletedAt != nil {
		resp["completedAt"] = task.CompletedAt
	}
	if task.Deadline != nil {
		resp["deadline"] = task.Deadline
	}
	if task.Remark != nil {
		resp["remark"] = task.Remark
	}
	return resp
}

func CreateTaskPlan(c *gin.Context) {
	var req struct {
		ParentID  *int         ``
		Name      string       ``
		PlanType  TaskPlanType ``
		CronExpr  *string      ``
		OrderNum  *int         ``
		Priority  *int         ``
		Remark    *string      ``
		Link      *string      ``
		StartedAt *time.Time   ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.PlanType == "" {
		req.PlanType = TaskPlanTypeTodo
	}

	if req.PlanType == TaskPlanTypeTodo && req.StartedAt == nil {
		renv.Error(c, http.StatusBadRequest, "待办类型必须传 startedAt")
		return
	}

	orderNum := 0
	if req.OrderNum != nil {
		orderNum = *req.OrderNum
	}
	priority := 5
	if req.Priority != nil {
		priority = *req.Priority
	}

	plan := TaskPlan{
		ParentID: req.ParentID,
		Name:     req.Name,
		Status:   TaskPlanStatusCreated,
		PlanType: req.PlanType,
		CronExpr: req.CronExpr,
		OrderNum: orderNum,
		Priority: priority,
		Remark:   req.Remark,
		Link:     req.Link,
	}

	if err := config.GetDB().Create(&plan).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "创建任务计划失败: " + err.Error())
		return
	}

	resp := gin.H{
		"id":        plan.ID,
		"parentId":  plan.ParentID,
		"name":      plan.Name,
		"status":    plan.Status,
		"planType":  plan.PlanType,
		"cronExpr":  plan.CronExpr,
		"orderNum":  plan.OrderNum,
		"priority":  plan.Priority,
		"remark":    plan.Remark,
		"link":      plan.Link,
		"createdAt": plan.CreatedAt,
		"updatedAt": plan.UpdatedAt,
	}

	if plan.PlanType == TaskPlanTypeTodo {
		firstTask, err := generateTask(&plan, *req.StartedAt, nil)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "生成任务失败: " + err.Error())
			return
		}
		resp["firstTask"] = buildTaskResponse(*firstTask)
	}

	renv.Success(c, resp)
}

func ListTaskPlans(c *gin.Context) {
	db := config.GetDB().Model(&TaskPlan{}).Where("is_deleted = ?", false)

	if planType := c.Query("planType"); planType != "" {
		db = db.Where("plan_type = ?", planType)
	}
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}

	var plans []TaskPlan
	if err := db.Order("order_num ASC, priority DESC, created_at DESC, id DESC").Find(&plans).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	renv.Success(c, plans)
}

func GetTaskPlanTree(c *gin.Context) {
	var plans []TaskPlan
	if err := config.GetDB().Select("ID", "ParentID", "Name", "Status", "PlanType", "TaskCount", "Priority", "OrderNum", "Link", "IsSuspended", "FsrsReps").
		Where("is_deleted = ?", false).Find(&plans).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	if keyword := strings.TrimSpace(c.Query("search")); keyword != "" {
		lower := strings.ToLower(keyword)
		byID := make(map[int]TaskPlan, len(plans))
		for _, p := range plans {
			byID[p.ID] = p
		}

		keep := make(map[int]bool)
		markWithAncestors := func(start TaskPlan) {
			cur := start
			for {
				if keep[cur.ID] {
					break
				}
				keep[cur.ID] = true
				if cur.ParentID == nil {
					break
				}
				parent, ok := byID[*cur.ParentID]
				if !ok {
					break
				}
				cur = parent
			}
		}
		for _, p := range plans {
			name := strings.ToLower(p.Name)
			remark := ""
			if p.Remark != nil {
				remark = strings.ToLower(*p.Remark)
			}
			if strings.Contains(name, lower) || strings.Contains(remark, lower) {
				markWithAncestors(p)
			}
		}

		filtered := make([]TaskPlan, 0, len(keep))
		for _, p := range plans {
			if keep[p.ID] {
				filtered = append(filtered, p)
			}
		}
		plans = filtered
	}

	tree := buildTaskPlanTree(plans)
	renv.Success(c, tree)
}

func GetTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	renv.Success(c, plan)
}

func UpdateTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var req struct {
		Name     *string      ``
		ParentID *int         ``
		PlanType TaskPlanType ``
		OrderNum *int         ``
		Priority *int         ``
		Remark   *string      ``
		Link     *string      ``
		CronExpr *string      ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	updated := make(map[string]interface{})
	if req.Name != nil {
		updated["name"] = *req.Name
	}
	if req.ParentID != nil {
		updated["parent_id"] = *req.ParentID
	} else {
		updated["parent_id"] = nil
	}
	updated["plan_type"] = req.PlanType
	if req.OrderNum != nil {
		updated["order_num"] = *req.OrderNum
	}
	if req.Priority != nil {
		updated["priority"] = *req.Priority
	}
	if req.Remark != nil {
		updated["remark"] = *req.Remark
	}
	if req.Link != nil {
		updated["link"] = *req.Link
	}
	if req.CronExpr != nil {
		updated["cron_expr"] = *req.CronExpr
	}
	if len(updated) == 0 {
		renv.Success(c, plan)
		return
	}
	updated["updated_at"] = time.Now()

	if err := config.GetDB().Model(&plan).Updates(updated).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}
	if err := config.GetDB().First(&plan, plan.ID).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取失败: " + err.Error())
		return
	}

	renv.Success(c, plan)
}

func StartTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.Status != TaskPlanStatusCreated && plan.Status != TaskPlanStatusStarted {
		renv.Error(c, http.StatusBadRequest, "当前状态不允许开启")
		return
	}

	if plan.IsSuspended {
		renv.Error(c, http.StatusBadRequest, "计划已挂起，请先恢复再开启")
		return
	}

	plan.Status = TaskPlanStatusStarted
	plan.UpdatedAt = time.Now()
	if err := config.GetDB().Save(&plan).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}

	var activeCount int64
	config.GetDB().Model(&Task{}).Where("plan_id = ? AND status = ? AND is_deleted = ?", plan.ID, TaskStatusActive, false).Count(&activeCount)

	var respTask *Task
	var resp map[string]interface{}
	if activeCount == 0 {
		var err error
		respTask, err = generateTask(&plan, time.Now(), nil)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "生成任务失败: " + err.Error())
			return
		}
	}

	resp = map[string]interface{}{
		"id":        plan.ID,
		"name":      plan.Name,
		"status":    plan.Status,
		"planType":  plan.PlanType,
		"updatedAt": plan.UpdatedAt,
	}
	if respTask != nil {
		resp["firstTask"] = buildTaskResponse(*respTask)
	}

	renv.Success(c, resp)
}

func CompleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.Status != TaskPlanStatusStarted {
		renv.Error(c, http.StatusBadRequest, "当前状态不允许完成")
		return
	}

	now := time.Now()
	db := config.GetDB().Begin()
	err := db.Model(&Task{}).Where("plan_id = ? AND status = ? AND is_deleted = ?", plan.ID, TaskStatusActive, false).Updates(map[string]interface{}{
		"status":       TaskStatusDone,
		"completed_at": now,
	}).Error
	if err != nil {
		db.Rollback()
		renv.Error(c, http.StatusInternalServerError, "更新任务失败: " + err.Error())
		return
	}

	plan.Status = TaskPlanStatusCompleted
	plan.UpdatedAt = now
	if err := db.Save(&plan).Error; err != nil {
		db.Rollback()
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}
	db.Commit()

	renv.Success(c, plan)
}

func ArchiveTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.Status != TaskPlanStatusCompleted {
		renv.Error(c, http.StatusBadRequest, "当前状态不允许归档")
		return
	}

	plan.Status = TaskPlanStatusArchived
	plan.UpdatedAt = time.Now()
	if err := config.GetDB().Save(&plan).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}

	renv.Success(c, plan)
}

func DeleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	cascade := c.Query("cascade") == "true"
	db := config.GetDB()

	var plan TaskPlan
	if err := db.Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	var childCount int64
	if err := db.Model(&TaskPlan{}).
		Where("parent_id = ? AND is_deleted = ?", id, false).
		Count(&childCount).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "检查子任务失败: " + err.Error())
		return
	}

	var activeTaskCount int64
	if err := db.Model(&Task{}).
		Where("plan_id = ? AND status = ? AND is_deleted = ?", id, TaskStatusActive, false).
		Count(&activeTaskCount).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "检查任务失败: " + err.Error())
		return
	}

	if !cascade {
		if childCount > 0 {
			renv.Error(c, http.StatusBadRequest, "存在子任务计划，无法删除")
			return
		}
		if activeTaskCount > 0 {
			renv.Error(c, http.StatusBadRequest, "存在未完成的任务，无法删除")
			return
		}
	}

	var allPlanIDs []int
	var collectPlanIDs func(int) error
	collectPlanIDs = func(planID int) error {
		allPlanIDs = append(allPlanIDs, planID)
		var children []TaskPlan
		if err := db.Where("parent_id = ? AND is_deleted = ?", planID, false).Find(&children).Error; err != nil {
			return err
		}
		for _, child := range children {
			if err := collectPlanIDs(child.ID); err != nil {
				return err
			}
		}
		return nil
	}

	tx := db.Begin()

	if cascade {
		if err := collectPlanIDs(id); err != nil {
			tx.Rollback()
			renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
			return
		}

		if len(allPlanIDs) > 0 {
			if err := tx.Model(&Task{}).
				Where("plan_id IN ?", allPlanIDs).
				Update("is_deleted", true).Error; err != nil {
				tx.Rollback()
				renv.Error(c, http.StatusInternalServerError, "删除任务失败: " + err.Error())
				return
			}
		}

		if err := tx.Model(&TaskPlan{}).
			Where("id IN ?", allPlanIDs).
			Updates(map[string]interface{}{
				"is_deleted": true,
				"updated_at": time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			renv.Error(c, http.StatusInternalServerError, "删除任务计划失败: " + err.Error())
			return
		}
	} else {
		if err := tx.Model(&Task{}).
			Where("plan_id = ?", id).
			Update("is_deleted", true).Error; err != nil {
			tx.Rollback()
			renv.Error(c, http.StatusInternalServerError, "删除任务失败: " + err.Error())
			return
		}

		if err := tx.Model(&plan).
			Updates(map[string]interface{}{
				"is_deleted": true,
				"updated_at": time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			renv.Error(c, http.StatusInternalServerError, "删除任务计划失败: " + err.Error())
			return
		}
	}

	tx.Commit()

	resp := gin.H{"message": "删除成功", "cascade": cascade}
	if cascade {
		resp["deletedPlanCount"] = len(allPlanIDs)
	}
	renv.Success(c, resp)
}

func SuspendTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"is_suspended": true,
			"updated_at":   now,
		}).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "挂起失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func ResumeTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"is_suspended": false,
			"updated_at":   now,
		}).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "恢复失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func SetPriorityTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var req struct {
		Priority int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "优先级必须为整数")
		return
	}
	if req.Priority < 0 {
		renv.Error(c, http.StatusBadRequest, "优先级不能为负数")
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"priority":   req.Priority,
			"updated_at": now,
		}).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新优先级失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func ListPlanTasks(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var tasks []Task
	if err := config.GetDB().Where("plan_id = ? AND is_deleted = ?", id, false).Order("created_at DESC, id DESC").Find(&tasks).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	renv.Success(c, tasks)
}

func GetPendingTasks(c *gin.Context) {
	now := time.Now()
	early := c.Query("early") == "1"

	// 基础查询（不含排序/分页参数），供 count 与分页查询复用
	base := config.GetDB().Table("tasks").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ?", TaskStatusActive).
		Where("tasks.is_deleted = ?", false).
		Where("task_plans.is_deleted = ?", false).
		Where("task_plans.is_suspended = ?", false)

	if !early {
		base = base.Where("(tasks.started_at IS NULL OR tasks.started_at <= ?)", now)
	}

	// 可选：按任务计划筛选（含该计划的全部子孙计划）
	if raw := strings.TrimSpace(c.Query("planId")); raw != "" {
		planID, err := strconv.Atoi(raw)
		if err != nil || planID <= 0 {
			renv.Error(c, http.StatusBadRequest, "无效的计划ID")
			return
		}
		planIDs, err := collectPlanWithDescendants(planID)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "收集子计划失败: " + err.Error())
			return
		}
		base = base.Where("tasks.plan_id IN ?", planIDs)
	}

	// 可选：按任务名称（即计划名）模糊筛选
	if name := strings.TrimSpace(c.Query("name")); name != "" {
		base = base.Where("task_plans.name LIKE ?", "%"+name+"%")
	}

	q := pagination.Parse(c)

	// 排序：默认按「优先级↓、截止↑(空置后)、开始↑」；支持前端 sort/order 覆盖。
	// 仅白名单列允许排序，规避 SQL 注入（方向仅拼接常量 ASC/DESC）。
	orderClause := "task_plans.priority DESC, tasks.deadline ASC NULLS LAST, tasks.started_at ASC"
	if sf := c.Query("sort"); sf != "" {
		allowed := map[string]string{
			"started_at": "tasks.started_at",
			"deadline":   "tasks.deadline",
			"plan_name":  "task_plans.name",
		}
		if col, ok := allowed[sf]; ok {
			dir := "ASC"
			if c.Query("order") == "desc" {
				dir = "DESC"
			}
			if sf == "deadline" {
				col += " NULLS LAST"
			}
			orderClause = col + " " + dir
		}
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
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
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	result := make([]PendingTask, 0, len(rows))
	for _, row := range rows {
		pt := PendingTask{
			Task:        row.Task,
			PlanName:    row.PlanName,
			PlanType:    row.PlanType,
			Link:        row.PlanLink,
			RawLink:     row.PlanRawLink,
			ContentSize: row.ContentSize,
			FsrsReps:    row.FsrsReps,
			IsOverdue:   row.Deadline != nil && now.After(*row.Deadline),
		}
		result = append(result, pt)
	}

	// 统一分页响应：{ items, total, page, size }
	renv.Success(c, pagination.New(result, total, q))
}

// finishTask 标记任务完成、驱动 FSRS（interval 类型）并生成下一条任务。
// 返回生成的下一条任务（可能为 nil）。task、plan 需已加载且 plan 已包含在事务外。
func finishTask(task *Task, plan *TaskPlan, rating *FsrsRating) (*Task, error) {
	db := config.GetDB()
	now := time.Now()
	task.Status = TaskStatusDone
	task.CompletedAt = &now
	if err := db.Save(task).Error; err != nil {
		return nil, err
	}

	var nextTask *Task
	switch plan.PlanType {
	case TaskPlanTypeCron, TaskPlanTypeInterval:
		if plan.IsSuspended {
			break
		}
		generated, err := generateTask(plan, now, rating)
		if err != nil {
			return nil, err
		}
		nextTask = generated
	}

	if plan.PlanType == TaskPlanTypeInterval && rating != nil {
		if err := db.Save(plan).Error; err != nil {
			return nil, err
		}
	}

	return nextTask, nil
}

func CompleteTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var req struct {
		Rating *int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Rating = nil
	}

	var task Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	if task.Status != TaskStatusActive {
		renv.Error(c, http.StatusBadRequest, "任务已完成")
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "所属任务计划不存在")
		return
	}

	var rating *FsrsRating
	if plan.PlanType == TaskPlanTypeInterval {
		if req.Rating == nil {
			renv.Error(c, http.StatusBadRequest, "interval 类型任务必须传 rating，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy")
			return
		}
		r := FsrsRating(*req.Rating)
		if r < RatingAgain || r > RatingEasy {
			renv.Error(c, http.StatusBadRequest, "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy")
			return
		}
		rating = &r
	}

	nextTask, err := finishTask(&task, &plan, rating)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "完成任务失败: " + err.Error())
		return
	}

	resp := buildTaskResponse(task)
	if nextTask != nil {
		resp["nextTask"] = buildTaskResponse(*nextTask)
	}

	renv.Success(c, resp)
}


// computePostponeUpdates 计算单条任务的延期更新字段，返回 updates、是否可延期与跳过原因。
// 仅 active 状态且所属 plan 为 todo/interval 类型的任务可延期，平移 started_at 与 deadline。
func computePostponeUpdates(task *Task, days int) (map[string]interface{}, bool, string) {
	if task.Status != TaskStatusActive {
		return nil, false, "任务已完成或已取消"
	}
	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		return nil, false, "所属任务计划不存在"
	}
	if plan.PlanType != TaskPlanTypeTodo && plan.PlanType != TaskPlanTypeInterval {
		return nil, false, "仅待办和间隔类型任务支持延期"
	}
	offset := time.Duration(days) * 24 * time.Hour
	updates := map[string]interface{}{}
	if task.StartedAt != nil {
		updates["started_at"] = task.StartedAt.Add(offset)
	}
	if task.Deadline != nil {
		updates["deadline"] = task.Deadline.Add(offset)
	}
	if len(updates) == 0 {
		return nil, false, "任务没有可延期的时间"
	}
	return updates, true, ""
}

func PostponeTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var req struct {
		Days int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days <= 0 {
		renv.Error(c, http.StatusBadRequest, "延期天数必须为正整数")
		return
	}

	var task Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	updates, ok2, reason := computePostponeUpdates(&task, req.Days)
	if !ok2 {
		renv.Error(c, http.StatusBadRequest, reason)
		return
	}

	if err := config.GetDB().Model(&task).Updates(updates).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "延期失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

// BatchPostponeTasks 批量延期：对同一组任务应用相同天数延期，逐条跳过不可延期的任务。
// 返回每条任务的处理结果（postponed / skipped 及跳过原因），便于前端展示部分成功。
func BatchPostponeTasks(c *gin.Context) {
	var req struct {
		IDs  []int `json:"ids"`
		Days int   `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days <= 0 {
		renv.Error(c, http.StatusBadRequest, "延期天数必须为正整数")
		return
	}
	if len(req.IDs) == 0 {
		renv.Error(c, http.StatusBadRequest, "请选择至少一个任务")
		return
	}

	db := config.GetDB()
	type ItemResult struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		Reason string `json:"reason,omitempty"`
	}
	results := make([]ItemResult, 0, len(req.IDs))
	postponed := 0
	skipped := 0

	for _, id := range req.IDs {
		var task Task
		if err := db.Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
			results = append(results, ItemResult{ID: id, Status: "skipped", Reason: "任务不存在"})
			skipped++
			continue
		}
		updates, ok, reason := computePostponeUpdates(&task, req.Days)
		if !ok {
			results = append(results, ItemResult{ID: id, Status: "skipped", Reason: reason})
			skipped++
			continue
		}
		if err := db.Model(&task).Updates(updates).Error; err != nil {
			renv.Error(c, http.StatusInternalServerError, "延期失败: " + err.Error())
			return
		}
		results = append(results, ItemResult{ID: id, Status: "postponed"})
		postponed++
	}

	renv.Success(c, nil)
}

func CancelTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var task Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	if task.Status != TaskStatusActive {
		renv.Error(c, http.StatusBadRequest, "任务已完成或已取消")
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "所属任务计划不存在")
		return
	}

	if plan.PlanType != TaskPlanTypeCron && plan.PlanType != TaskPlanTypeInterval {
		renv.Error(c, http.StatusBadRequest, "仅周期重复任务支持取消")
		return
	}

	now := time.Now()
	task.Status = TaskStatusCancelled
	task.CompletedAt = &now
	if err := config.GetDB().Save(&task).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}

	resp := buildTaskResponse(task)

	nextTask, err := generateTask(&plan, now, nil)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "生成下一条任务失败: " + err.Error())
		return
	}
	resp["nextTask"] = buildTaskResponse(*nextTask)

	renv.Success(c, resp)
}

