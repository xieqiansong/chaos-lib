package taskplan

import (
	renv "chaos-go/internal/resp"
	"chaos-go/internal/config"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GetTaskDailyStats(c *gin.Context) {
	db := config.GetDB()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	days, err := parseDaysParam(c, 29)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	cutoff := today.AddDate(0, 0, -days)

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
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	rowMap := make(map[string]int, len(rows))
	for _, r := range rows {
		local := r.CompletedAt.In(time.Local)
		key := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
		rowMap[key]++
	}

	result := make([]map[string]interface{}, 0, days+1)
	for i := days; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
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

	renv.Success(c, result)
}

// parseDaysParam 解析前端传入的 days（正整数），缺省时回退到默认值。
func parseDaysParam(c *gin.Context, def int) (int, error) {
	v := c.Query("days")
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def, fmt.Errorf("参数 days 必须为正整数")
	}
	return n, nil
}

func GetTaskActiveStats(c *gin.Context) {
	db := config.GetDB()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	start, end, err := parseRange(c, today)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	type Row struct {
		StartedAt time.Time
	}
	var rows []Row
	if err := db.Table("tasks").
		Select("started_at").
		Joins("JOIN task_plans ON task_plans.id = tasks.plan_id").
		Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.started_at IS NOT NULL AND tasks.started_at >= ? AND tasks.started_at < ?",
			TaskStatusActive, false, start, end.AddDate(0, 0, 1)).
		Where("task_plans.is_suspended = ?", false).
		Find(&rows).Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	rowMap := make(map[string]int, len(rows))
	for _, r := range rows {
		local := r.StartedAt.In(time.Local)
		key := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
		rowMap[key]++
	}

	result := make([]map[string]interface{}, 0, 30)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
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

	renv.Success(c, result)
}

// parseRange 解析前端传入的 start/end（YYYY-MM-DD），缺省时回退到默认范围
// （今天-6 天到今天+24 天），保持改动前的展示效果。
func parseRange(c *gin.Context, today time.Time) (start, end time.Time, err error) {
	defaultStart := today.AddDate(0, 0, -6)
	defaultEnd := today.AddDate(0, 0, 24)

	start, err = parseDateParam(c, "start", defaultStart)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err = parseDateParam(c, "end", defaultEnd)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("结束日期不能早于开始日期")
	}
	return start, end, nil
}

func parseDateParam(c *gin.Context, name string, def time.Time) (time.Time, error) {
	v := c.Query(name)
	if v == "" {
		return def, nil
	}
	t, err := time.ParseInLocation("2006-01-02", v, time.Local)
	if err != nil {
		return def, fmt.Errorf("参数 %s 格式应为 YYYY-MM-DD", name)
	}
	return t, nil
}

// buildContributionSeries 按天补齐 [start, today] 区间，生成单个统计项的贡献序列。
func buildContributionSeries(id int, name string, start, today time.Time, counts map[string]int) gin.H {
	days := make([]map[string]interface{}, 0, 366)
	total := 0
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		count := counts[key]
		total += count
		days = append(days, map[string]interface{}{
			"date":  key,
			"count": count,
		})
	}
	return gin.H{
		"id":    id,
		"name":  name,
		"total": total,
		"days":  days,
	}
}

// GetTaskContributionStats 返回近一年每日完成任务数（GitHub 风格贡献热力图）。
// 每天归属按任务的开始时间（started_at）划分，而非完成时间：定时/周期任务在当日达成就
// 应计入当日，即使实际完成动作跨到了第二天。
// 默认以名为「每日任务」的计划为根，取其直接子计划作为可切换的统计项（每个子项单独统计，
// 不合并）；子计划为空时退化为根计划自身。可用 ?planId= 直接指定根计划，
// 或用 ?rootName= 指定根计划名。
func GetTaskContributionStats(c *gin.Context) {
	db := config.GetDB()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	// 往前多取一天，由前端对齐到整周（周日）后渲染
	start := today.AddDate(-1, 0, 1)

	// 解析统计根计划：优先 planId，其次按名称（默认「每日任务」）
	var roots []TaskPlan
	if pid := strings.TrimSpace(c.Query("planId")); pid != "" {
		id, err := strconv.Atoi(pid)
		if err != nil {
			renv.Error(c, http.StatusBadRequest, "无效的 planId")
			return
		}
		var plan TaskPlan
		if err := db.Where("id = ? AND is_deleted = ? AND is_suspended = ?", id, false, false).First(&plan).Error; err != nil {
			renv.Error(c, http.StatusNotFound, "任务计划不存在")
			return
		}
		roots = append(roots, plan)
	} else {
		rootName := strings.TrimSpace(c.Query("rootName"))
		if rootName == "" {
			rootName = "每日任务"
		}
		if err := db.Where("name = ? AND is_deleted = ? AND is_suspended = ?", rootName, false, false).Find(&roots).Error; err != nil {
			renv.Error(c, http.StatusInternalServerError, "查询计划失败: " + err.Error())
			return
		}
	}

	// 可切换的统计项 = 根计划的直接子计划（保持与任务树一致的排序）
	options := make([]TaskPlan, 0, 8)
	rootName := ""
	for _, root := range roots {
		if rootName == "" {
			rootName = root.Name
		}
		var children []TaskPlan
		if err := db.Where("parent_id = ? AND is_deleted = ? AND is_suspended = ?", root.ID, false, false).
			Order("order_num ASC, id ASC").Find(&children).Error; err != nil {
			renv.Error(c, http.StatusInternalServerError, "查询子计划失败: " + err.Error())
			return
		}
		if len(children) == 0 {
			// 根计划没有子计划时，直接以自身作为唯一统计项
			options = append(options, root)
		} else {
			options = append(options, children...)
		}
	}

	// 归集各统计项的子树计划：planID -> 选项下标
	owner := make(map[int]int)
	for idx := range options {
		ids, err := collectPlanWithDescendants(options[idx].ID)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "收集子计划失败: " + err.Error())
			return
		}
		for _, id := range ids {
			owner[id] = idx
		}
	}

	counts := make([]map[string]int, len(options))
	for i := range counts {
		counts[i] = make(map[string]int)
	}

	if len(owner) > 0 {
		planIDs := make([]int, 0, len(owner))
		for id := range owner {
			planIDs = append(planIDs, id)
		}

		type Row struct {
			PlanID    int
			StartedAt time.Time
		}
		var rows []Row
		if err := db.Table("tasks").
			Select("tasks.plan_id, tasks.started_at").
			Where("tasks.status = ? AND tasks.is_deleted = ? AND tasks.started_at IS NOT NULL AND tasks.started_at >= ?",
				TaskStatusDone, false, start).
			Where("tasks.plan_id IN ?", planIDs).
			Find(&rows).Error; err != nil {
			renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
			return
		}

		for _, r := range rows {
			idx, ok := owner[r.PlanID]
			if !ok {
				continue
			}
			local := r.StartedAt.In(time.Local)
			key := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
			counts[idx][key]++
		}
	}

	items := make([]gin.H, 0, len(options))
	for i, opt := range options {
		items = append(items, buildContributionSeries(opt.ID, opt.Name, start, today, counts[i]))
	}

	renv.Success(c, gin.H{
		"rootName": rootName,
		"start":    start.Format("2006-01-02"),
		"end":      today.Format("2006-01-02"),
		"items":    items,
	})
}
