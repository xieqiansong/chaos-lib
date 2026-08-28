package taskplan

import (
	"chaos-go/config"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return 0, false
	}
	return id, true
}

func getTaskID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.PlanType == "" {
		req.PlanType = TaskPlanTypeTodo
	}

	if req.PlanType == TaskPlanTypeTodo && req.StartedAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "待办类型必须传 startedAt"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务计划失败: " + err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成任务失败: " + err.Error()})
			return
		}
		resp["firstTask"] = buildTaskResponse(*firstTask)
	}

	c.JSON(http.StatusCreated, resp)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, plans)
}

func GetTaskPlanTree(c *gin.Context) {
	var plans []TaskPlan
	if err := config.GetDB().Select("ID", "ParentID", "Name", "Status", "PlanType", "TaskCount", "Priority", "OrderNum", "Link").
		Where("is_deleted = ?", false).Find(&plans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
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
	c.JSON(http.StatusOK, tree)
}

func GetTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	c.JSON(http.StatusOK, plan)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
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
		c.JSON(http.StatusOK, plan)
		return
	}
	updated["updated_at"] = time.Now()

	if err := config.GetDB().Model(&plan).Updates(updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	if err := config.GetDB().First(&plan, plan.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
}

func StartTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	if plan.Status != TaskPlanStatusCreated && plan.Status != TaskPlanStatusStarted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不允许开启"})
		return
	}

	plan.Status = TaskPlanStatusStarted
	plan.UpdatedAt = time.Now()
	if err := config.GetDB().Save(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成任务失败: " + err.Error()})
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

	c.JSON(http.StatusOK, resp)
}

func CompleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	if plan.Status != TaskPlanStatusStarted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不允许完成"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新任务失败: " + err.Error()})
		return
	}

	plan.Status = TaskPlanStatusCompleted
	plan.UpdatedAt = now
	if err := db.Save(&plan).Error; err != nil {
		db.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	db.Commit()

	c.JSON(http.StatusOK, plan)
}

func ArchiveTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	if plan.Status != TaskPlanStatusCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不允许归档"})
		return
	}

	plan.Status = TaskPlanStatusArchived
	plan.UpdatedAt = time.Now()
	if err := config.GetDB().Save(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	var childCount int64
	if err := db.Model(&TaskPlan{}).
		Where("parent_id = ? AND is_deleted = ?", id, false).
		Count(&childCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查子任务失败: " + err.Error()})
		return
	}

	var activeTaskCount int64
	if err := db.Model(&Task{}).
		Where("plan_id = ? AND status = ? AND is_deleted = ?", id, TaskStatusActive, false).
		Count(&activeTaskCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查任务失败: " + err.Error()})
		return
	}

	if !cascade {
		if childCount > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "存在子任务计划，无法删除",
				"childCount": childCount,
				"suggestion": "请先删除子任务计划，或使用 ?cascade=true 级联删除",
			})
			return
		}
		if activeTaskCount > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "存在未完成的任务，无法删除",
				"activeTaskCount": activeTaskCount,
				"suggestion":      "请先完成任务，或使用 ?cascade=true 级联删除",
			})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "收集子任务失败: " + err.Error()})
			return
		}

		if len(allPlanIDs) > 0 {
			if err := tx.Model(&Task{}).
				Where("plan_id IN ?", allPlanIDs).
				Update("is_deleted", true).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务失败: " + err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务计划失败: " + err.Error()})
			return
		}
	} else {
		if err := tx.Model(&Task{}).
			Where("plan_id = ?", id).
			Update("is_deleted", true).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务失败: " + err.Error()})
			return
		}

		if err := tx.Model(&plan).
			Updates(map[string]interface{}{
				"is_deleted": true,
				"updated_at": time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务计划失败: " + err.Error()})
			return
		}
	}

	tx.Commit()

	resp := gin.H{"message": "删除成功", "cascade": cascade}
	if cascade {
		resp["deletedPlanCount"] = len(allPlanIDs)
	}
	c.JSON(http.StatusOK, resp)
}

func SuspendTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "收集子任务失败: " + err.Error()})
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"is_suspended": true,
			"updated_at":   now,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "挂起失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "已挂起",
		"suspendedID":   id,
		"affectedCount": len(ids),
	})
}

func ResumeTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "收集子任务失败: " + err.Error()})
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"is_suspended": false,
			"updated_at":   now,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "已恢复",
		"resumedID":     id,
		"affectedCount": len(ids),
	})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "优先级必须为整数"})
		return
	}
	if req.Priority < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "优先级不能为负数"})
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "收集子任务失败: " + err.Error()})
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"priority":   req.Priority,
			"updated_at": now,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新优先级失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "已更新优先级",
		"planID":        id,
		"priority":      req.Priority,
		"affectedCount": len(ids),
	})
}

func ListPlanTasks(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var tasks []Task
	if err := config.GetDB().Where("plan_id = ? AND is_deleted = ?", id, false).Order("created_at DESC, id DESC").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

func GetPendingTasks(c *gin.Context) {
	now := time.Now()
	early := c.Query("early") == "1"

	var rows []struct {
		Task
		PlanName    string       ``
		PlanType    TaskPlanType ``
		PlanLink    *string      ``
		ContentSize int          ``
		FsrsReps    int          ``
	}
	query := config.GetDB().Table("tasks").
		Select("tasks.*, task_plans.name AS plan_name, task_plans.plan_type AS plan_type, task_plans.link AS plan_link, task_plans.content_size, task_plans.fsrs_reps").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ?", TaskStatusActive).
		Where("tasks.is_deleted = ?", false).
		Where("task_plans.is_deleted = ?", false).
		Where("task_plans.is_suspended = ?", false)

	if !early {
		query = query.Where("(tasks.started_at IS NULL OR tasks.started_at <= ?)", now)
	}

	if err := query.
		Order("task_plans.priority DESC, tasks.deadline ASC NULLS LAST, tasks.started_at ASC").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	result := make([]PendingTask, 0, len(rows))
	for _, row := range rows {
		pt := PendingTask{
			Task:        row.Task,
			PlanName:    row.PlanName,
			PlanType:    row.PlanType,
			Link:        row.PlanLink,
			ContentSize: row.ContentSize,
			FsrsReps:    row.FsrsReps,
			IsOverdue:   row.Deadline != nil && now.After(*row.Deadline),
		}
		result = append(result, pt)
	}

	c.JSON(http.StatusOK, result)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != TaskStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已完成"})
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属任务计划不存在"})
		return
	}

	var rating *FsrsRating
	if plan.PlanType == TaskPlanTypeInterval {
		if req.Rating == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "interval 类型任务必须传 rating，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"})
			return
		}
		r := FsrsRating(*req.Rating)
		if r < RatingAgain || r > RatingEasy {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"})
			return
		}
		rating = &r
	}

	now := time.Now()
	task.Status = TaskStatusDone
	task.CompletedAt = &now
	if err := config.GetDB().Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	resp := buildTaskResponse(task)

	var nextTask *Task
	switch plan.PlanType {
	case TaskPlanTypeCron, TaskPlanTypeInterval:
		generated, err := generateTask(&plan, now, rating)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成下一条任务失败: " + err.Error()})
			return
		}
		nextTask = generated
		resp["nextTask"] = buildTaskResponse(*nextTask)
	}

	if plan.PlanType == TaskPlanTypeInterval && rating != nil {
		if err := config.GetDB().Save(&plan).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新FSRS状态失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

func GetTaskDailyStats(c *gin.Context) {
	db := config.GetDB()
	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -29)

	type Row struct {
		CompletedAt time.Time
	}
	var rows []Row
	if err := db.Table("tasks").
		Select("completed_at").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.completed_at IS NOT NULL AND tasks.completed_at >= ?",
			TaskStatusDone, false, cutoff).
		Where("task_plans.is_suspended = ?", false).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	rowMap := make(map[string]int, len(rows))
	for _, r := range rows {
		local := r.CompletedAt.In(time.Local)
		key := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
		rowMap[key]++
	}

	result := make([]map[string]interface{}, 0, 30)
	for i := 29; i >= 0; i-- {
		d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		count := 0
		if v, ok := rowMap[dateStr]; ok {
			count = v
		}
		result = append(result, map[string]interface{}{
			"date":  dateStr,
			"count": count,
		})
	}

	c.JSON(http.StatusOK, result)
}

func GetTaskActiveStats(c *gin.Context) {
	db := config.GetDB()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	cutoff := today.AddDate(0, 0, -6)

	type Row struct {
		StartedAt time.Time
	}
	var rows []Row
	if err := db.Table("tasks").
		Select("started_at").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.started_at IS NOT NULL AND tasks.started_at >= ?",
			TaskStatusActive, false, cutoff).
		Where("task_plans.is_suspended = ?", false).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	rowMap := make(map[string]int, len(rows))
	for _, r := range rows {
		local := r.StartedAt.In(time.Local)
		key := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
		rowMap[key]++
	}

	result := make([]map[string]interface{}, 0, 30)
	for i := -6; i <= 24; i++ {
		d := today.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		count := 0
		if v, ok := rowMap[dateStr]; ok {
			count = v
		}
		result = append(result, map[string]interface{}{
			"date":  dateStr,
			"count": count,
		})
	}

	c.JSON(http.StatusOK, result)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "延期天数必须为正整数"})
		return
	}

	var task Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != TaskStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已完成或已取消"})
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属任务计划不存在"})
		return
	}

	if plan.PlanType != TaskPlanTypeTodo && plan.PlanType != TaskPlanTypeInterval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅待办和间隔类型任务支持延期"})
		return
	}

	offset := time.Duration(req.Days) * 24 * time.Hour
	updates := map[string]interface{}{}

	if task.StartedAt != nil {
		newStarted := task.StartedAt.Add(offset)
		updates["started_at"] = newStarted
	}
	if task.Deadline != nil {
		newDeadline := task.Deadline.Add(offset)
		updates["deadline"] = newDeadline
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务没有可延期的时间"})
		return
	}

	if err := config.GetDB().Model(&task).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "延期失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已延期 %d 天", req.Days),
		"days":    req.Days,
	})
}

func CancelTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var task Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != TaskStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已完成或已取消"})
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属任务计划不存在"})
		return
	}

	if plan.PlanType != TaskPlanTypeCron && plan.PlanType != TaskPlanTypeInterval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅周期重复任务支持取消"})
		return
	}

	now := time.Now()
	task.Status = TaskStatusCancelled
	task.CompletedAt = &now
	if err := config.GetDB().Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	resp := buildTaskResponse(task)

	nextTask, err := generateTask(&plan, now, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成下一条任务失败: " + err.Error()})
		return
	}
	resp["nextTask"] = buildTaskResponse(*nextTask)

	c.JSON(http.StatusOK, resp)
}
