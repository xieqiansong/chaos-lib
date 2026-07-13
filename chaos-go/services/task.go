package services

import (
	"chaos-lib/config"
	"chaos-lib/models"
	"chaos-lib/tools"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

var fsrsInstance = tools.NewFsrs(nil)

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

// collectPlanWithDescendants 收集 rootID 及其所有子孙任务计划的 ID（含自身）
func collectPlanWithDescendants(rootID int) ([]int, error) {
	db := config.GetDB()
	ids := []int{}
	var walk func(int) error
	walk = func(id int) error {
		ids = append(ids, id)
		var children []models.TaskPlan
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

func buildTaskPlanTree(plans []models.TaskPlan) []models.TaskPlanTree {
	childrenMap := make(map[int][]models.TaskPlan)
	var roots []models.TaskPlan

	for _, p := range plans {
		if p.ParentID == nil {
			roots = append(roots, p)
		} else {
			childrenMap[*p.ParentID] = append(childrenMap[*p.ParentID], p)
		}
	}

	sortByOrder := func(list []models.TaskPlan) {
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

	var build func(parent models.TaskPlan) models.TaskPlanTree
	build = func(parent models.TaskPlan) models.TaskPlanTree {
		node := models.TaskPlanTree{TaskPlan: parent}
		for _, child := range childrenMap[parent.ID] {
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	var result []models.TaskPlanTree
	for _, root := range roots {
		result = append(result, build(root))
	}
	return result
}

// generateTask 根据任务计划生成一条任务
// rating 仅对 interval 类型计划有意义：当 rating 不为 nil 时，按 FSRS 算法调度下一条任务并更新 plan 的 FSRS 状态
func generateTask(plan *models.TaskPlan, now time.Time, rating *tools.FsrsRating) (*models.Task, error) {
	now = now.Truncate(time.Second)
	task := models.Task{
		PlanID:    plan.ID,
		Status:    models.TaskStatusActive,
		StartedAt: &now,
	}

	switch plan.PlanType {
	case models.TaskPlanTypeCron:
		if plan.CronExpr == nil || *plan.CronExpr == "" {
			return nil, fmt.Errorf("cron表达式不能为空")
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(*plan.CronExpr)
		if err != nil {
			return nil, fmt.Errorf("无效的cron表达式: %v", err)
		}
		return createCronTask(plan, schedule.Next(now))

	case models.TaskPlanTypeInterval:
		if rating != nil {
			fsrsCard := tools.FsrsCard{
				Stability:     plan.FsrsStability,
				Difficulty:    plan.FsrsDifficulty,
				Reps:          plan.FsrsReps,
				Lapses:        plan.FsrsLapses,
				State:         tools.FsrsState(plan.FsrsState),
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

func buildTaskResponse(task models.Task) gin.H {
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

// CreateTaskPlan 创建任务计划
// 同时根据任务类型生成第一条任务记录
func CreateTaskPlan(c *gin.Context) {
	var req struct {
		ParentID  *int                ``
		Name      string              ``
		PlanType  models.TaskPlanType ``
		CronExpr  *string             ``
		OrderNum  *int                ``
		Priority  *int                ``
		Remark    *string             ``
		Link      *string             ``
		StartedAt *time.Time          ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.PlanType == "" {
		req.PlanType = models.TaskPlanTypeTodo
	}

	if req.PlanType == models.TaskPlanTypeTodo && req.StartedAt == nil {
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

	plan := models.TaskPlan{
		ParentID: req.ParentID,
		Name:     req.Name,
		Status:   models.TaskPlanStatusCreated,
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

	// todo 类型为一次性事项，创建后立即生成一条任务；
	// cron / interval 等周期任务需等到 StartTaskPlan 开启计划后才生成第一条任务，
	// 避免 plan 状态仍为 created 时就出现 tasks。
	if plan.PlanType == models.TaskPlanTypeTodo {
		firstTask, err := generateTask(&plan, *req.StartedAt, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成任务失败: " + err.Error()})
			return
		}
		resp["firstTask"] = buildTaskResponse(*firstTask)
	}

	c.JSON(http.StatusCreated, resp)
}

// ListTaskPlans 查询任务计划列表
// 支持按 planType、status 过滤
func ListTaskPlans(c *gin.Context) {
	db := config.GetDB().Model(&models.TaskPlan{}).Where("is_deleted = ?", false)

	if planType := c.Query("planType"); planType != "" {
		db = db.Where("plan_type = ?", planType)
	}
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}

	var plans []models.TaskPlan
	if err := db.Order("order_num ASC, priority DESC, created_at DESC, id DESC").Find(&plans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, plans)
}

// GetTaskPlanTree 查询任务计划树
// 递归组装树形结构；支持 ?search= 按名称/备注模糊匹配，并保留命中节点的全部祖先以保证树连通
func GetTaskPlanTree(c *gin.Context) {
	var plans []models.TaskPlan
	if err := config.GetDB().Where("is_deleted = ?", false).Find(&plans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	if keyword := strings.TrimSpace(c.Query("search")); keyword != "" {
		lower := strings.ToLower(keyword)
		byID := make(map[int]models.TaskPlan, len(plans))
		for _, p := range plans {
			byID[p.ID] = p
		}

		keep := make(map[int]bool)
		markWithAncestors := func(start models.TaskPlan) {
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

		filtered := make([]models.TaskPlan, 0, len(keep))
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

// GetTaskPlan 查询单条任务计划
func GetTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// UpdateTaskPlan 更新任务计划的基础字段（名称、顺序、优先级、备注、链接）
func UpdateTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var req struct {
		Name     *string             ``
		ParentID *int                ``
		PlanType models.TaskPlanType ``
		OrderNum *int                ``
		Priority *int                ``
		Remark   *string             ``
		Link     *string             ``
		CronExpr *string             ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var plan models.TaskPlan
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

// StartTaskPlan 开启任务计划
// 状态流转: created/started -> started
// 同时根据任务计划类型生成对应的任务记录
func StartTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	if plan.Status != models.TaskPlanStatusCreated && plan.Status != models.TaskPlanStatusStarted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不允许开启"})
		return
	}

	plan.Status = models.TaskPlanStatusStarted
	plan.UpdatedAt = time.Now()
	if err := config.GetDB().Save(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	// 如果计划下没有待处理任务，生成第一条任务
	var activeCount int64
	config.GetDB().Model(&models.Task{}).Where("plan_id = ? AND status = ? AND is_deleted = ?", plan.ID, models.TaskStatusActive, false).Count(&activeCount)

	var respTask *models.Task
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

// CompleteTaskPlan 完成任务计划
// 状态流转: started -> completed
// 同时把计划下所有未完成的任务标记为已完成
func CompleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	if plan.Status != models.TaskPlanStatusStarted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不允许完成"})
		return
	}

	// 使用服务器本地时区（非 UTC），与前端显示一致
	now := time.Now()
	db := config.GetDB().Begin()
	err := db.Model(&models.Task{}).Where("plan_id = ? AND status = ? AND is_deleted = ?", plan.ID, models.TaskStatusActive, false).Updates(map[string]interface{}{
		"status":       models.TaskStatusDone,
		"completed_at": now,
	}).Error
	if err != nil {
		db.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新任务失败: " + err.Error()})
		return
	}

	plan.Status = models.TaskPlanStatusCompleted
	plan.UpdatedAt = now
	if err := db.Save(&plan).Error; err != nil {
		db.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	db.Commit()

	c.JSON(http.StatusOK, plan)
}

// ArchiveTaskPlan 归档任务计划
// 状态流转: completed -> archived
func ArchiveTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	if plan.Status != models.TaskPlanStatusCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前状态不允许归档"})
		return
	}

	plan.Status = models.TaskPlanStatusArchived
	plan.UpdatedAt = time.Now()
	if err := config.GetDB().Save(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// DeleteTaskPlan 删除任务计划（软删除）
// 通过 query 参数 cascade 控制：
//   - cascade=false（默认）：如果有子任务计划或未完成任务，不允许删除
//   - cascade=true：级联删除该计划及其所有子任务计划和任务
func DeleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	cascade := c.Query("cascade") == "true"

	db := config.GetDB()

	var plan models.TaskPlan
	if err := db.Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	// 检查是否有子任务计划
	var childCount int64
	if err := db.Model(&models.TaskPlan{}).
		Where("parent_id = ? AND is_deleted = ?", id, false).
		Count(&childCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查子任务失败: " + err.Error()})
		return
	}

	// 检查是否有未完成的任务
	var activeTaskCount int64
	if err := db.Model(&models.Task{}).
		Where("plan_id = ? AND status = ? AND is_deleted = ?", id, models.TaskStatusActive, false).
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

	// 收集需要删除的所有任务计划 ID（包括自身和所有子孙）
	var allPlanIDs []int
	var collectPlanIDs func(int) error
	collectPlanIDs = func(planID int) error {
		allPlanIDs = append(allPlanIDs, planID)
		var children []models.TaskPlan
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

		// 级联软删除所有相关任务
		if len(allPlanIDs) > 0 {
			if err := tx.Model(&models.Task{}).
				Where("plan_id IN ?", allPlanIDs).
				Update("is_deleted", true).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务失败: " + err.Error()})
				return
			}
		}

		// 级联软删除所有任务计划
		if err := tx.Model(&models.TaskPlan{}).
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
		// 仅删除当前计划和其下的所有任务（已确认无子计划和未完成任务）
		if err := tx.Model(&models.Task{}).
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

// SuspendTaskPlan 挂起（暂停）任务计划
// 递归将其本身及所有子孙任务计划标记为 is_suspended = true，
// 被挂起的计划不会出现在待办任务列表中（GetPendingTasks 已过滤）。
// 主要用于暂时不想继续、但后续会恢复的任务。
func SuspendTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	// 校验计划存在
	var plan models.TaskPlan
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
	if err := config.GetDB().Model(&models.TaskPlan{}).
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

// ResumeTaskPlan 恢复（启动）任务计划
// 递归将其本身及所有子孙任务计划标记为 is_suspended = false。
func ResumeTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan models.TaskPlan
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
	if err := config.GetDB().Model(&models.TaskPlan{}).
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

// SetPriorityTaskPlan 递归修改任务计划及其所有子孙任务计划的优先级
// 用于一次性调整整棵子树的重要程度。
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

	// 校验计划存在
	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务计划不存在"})
		return
	}

	// 递归收集自身及所有子孙任务计划的 ID
	ids, err := collectPlanWithDescendants(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "收集子任务失败: " + err.Error()})
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&models.TaskPlan{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"priority":   req.Priority,
			"updated_at": now,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新优先级失败: " + err.Error()})
		return
	}

	// 待办任务的排序由 task_plans.priority 决定（GetPendingTasks 已改为按
	// task_plans.priority 排序），无需再同步 tasks 表。

	c.JSON(http.StatusOK, gin.H{
		"message":       "已更新优先级",
		"planID":        id,
		"priority":      req.Priority,
		"affectedCount": len(ids),
	})
}

// ListPlanTasks 查询任务计划下的任务列表
func ListPlanTasks(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var tasks []models.Task
	if err := config.GetDB().Where("plan_id = ? AND is_deleted = ?", id, false).Order("created_at DESC, id DESC").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// GetPendingTasks 查询待办任务
// 查询所有状态为 active 的任务（含过期任务，通过 IsOverdue 标记）。
// 默认只返回当前时间已到达 started_at 的任务；传 ?early=1 可提前查看所有 active 任务（不限开始时间）。
func GetPendingTasks(c *gin.Context) {
	// 使用服务器本地时区，与前端显示一致
	now := time.Now()
	early := c.Query("early") == "1"

	var rows []struct {
		models.Task
		PlanName    string              ``
		PlanType    models.TaskPlanType ``
		PlanLink    *string             ``
		ContentSize int                 ``
	}
	query := config.GetDB().Table("tasks").
		Select("tasks.*, task_plans.name AS plan_name, task_plans.plan_type AS plan_type, task_plans.link AS plan_link, task_plans.content_size").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ?", models.TaskStatusActive).
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

	result := make([]models.PendingTask, 0, len(rows))
	for _, row := range rows {
		pt := models.PendingTask{
			Task:        row.Task,
			PlanName:    row.PlanName,
			PlanType:    row.PlanType,
			Link:        row.PlanLink,
			ContentSize: row.ContentSize,
			IsOverdue:   row.Deadline != nil && now.After(*row.Deadline),
		}
		result = append(result, pt)
	}

	c.JSON(http.StatusOK, result)
}

// CompleteTask 完成单条任务
// 将任务状态改为 done，并根据任务计划类型生成下一条任务
// 对于 interval 类型计划，必须传 rating（1=Again, 2=Hard, 3=Good, 4=Easy），用于 FSRS 调度
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

	var task models.Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != models.TaskStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已完成"})
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属任务计划不存在"})
		return
	}

	// 对于 interval 类型，rating 为必填参数
	var rating *tools.FsrsRating
	if plan.PlanType == models.TaskPlanTypeInterval {
		if req.Rating == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "interval 类型任务必须传 rating，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"})
			return
		}
		r := tools.FsrsRating(*req.Rating)
		if r < tools.FsrsRatingAgain || r > tools.FsrsRatingEasy {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"})
			return
		}
		rating = &r
	}

	// 使用服务器本地时区，与前端显示一致
	now := time.Now()
	task.Status = models.TaskStatusDone
	task.CompletedAt = &now
	if err := config.GetDB().Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	resp := buildTaskResponse(task)

	var nextTask *models.Task
	switch plan.PlanType {
	case models.TaskPlanTypeCron, models.TaskPlanTypeInterval:
		generated, err := generateTask(&plan, now, rating)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成下一条任务失败: " + err.Error()})
			return
		}
		nextTask = generated
		resp["nextTask"] = buildTaskResponse(*nextTask)
	}

	// interval 类型下，generateTask 已根据 rating 更新了 plan 的 FSRS 状态，这里持久化
	if plan.PlanType == models.TaskPlanTypeInterval && rating != nil {
		if err := config.GetDB().Save(&plan).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新FSRS状态失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetTaskDailyStats 统计最近30天每天完成的任务数
// 注意：按应用本地时区（time.Local）对 completed_at 做日期归桶，
// 避免依赖数据库会话时区（如 UTC）导致跨天边界 off-by-one。
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
			models.TaskStatusDone, false, cutoff).
		Where("task_plans.is_suspended = ?", false).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	// 按本地日期归桶（与时区无关，统一以 time.Local 切分）
	rowMap := make(map[string]int, len(rows))
	for _, r := range rows {
		local := r.CompletedAt.In(time.Local)
		key := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
		rowMap[key]++
	}

	// 补全缺失的日期（count = 0）
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

// GetTaskActiveStats 统计过去6天逾期 + 未来24天待办（active）的任务数（按 started_at 分组，总计30天含今天）
// 同样按应用本地时区归桶，避免数据库会话时区导致边界错位。
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
			models.TaskStatusActive, false, cutoff).
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

	// -6 ~ +24天，共30天（含今天）
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

// PostponeTask 延期单条任务
// 仅对 todo/interval 类型有效，将 startedAt 向后推延指定天数
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

	var task models.Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != models.TaskStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已完成或已取消"})
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属任务计划不存在"})
		return
	}

	if plan.PlanType != models.TaskPlanTypeTodo && plan.PlanType != models.TaskPlanTypeInterval {
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

// CancelTask 取消单条任务
// 仅对 cron/interval 类型的周期重复任务有效，将任务状态改为 cancelled 并生成下一条任务
// todo 类型的任务不支持取消（应直接完成或删除）
func CancelTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var task models.Task
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != models.TaskStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已完成或已取消"})
		return
	}

	var plan models.TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", task.PlanID, false).First(&plan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属任务计划不存在"})
		return
	}

	if plan.PlanType != models.TaskPlanTypeCron && plan.PlanType != models.TaskPlanTypeInterval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅周期重复任务支持取消"})
		return
	}

	now := time.Now()
	task.Status = models.TaskStatusCancelled
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
