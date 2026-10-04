package taskplan

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"github.com/gin-gonic/gin"
)

// GetTaskDailyStats 每日完成任务数（默认最近 30 天，可用 ?days= 指定）。
func GetTaskDailyStats(c *gin.Context) {
	days, err := parseDaysParam(c, 29)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	points, err := DailyStats(days)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, points)
}

// GetTaskActiveStats 区间内每日「任务开始」数（?start / ?end，YYYY-MM-DD）。
func GetTaskActiveStats(c *gin.Context) {
	start, end, err := parseRange(c, todayStart())
	if err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	points, err := ActiveStats(start, end)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, points)
}

// GetTaskContributionStats 近一年每日完成任务数（GitHub 风格贡献热力图）。
// 可用 ?planId= 指定根计划，或用 ?rootName= 指定根计划名（默认「每日任务」）。
func GetTaskContributionStats(c *gin.Context) {
	planID := 0
	if pid := strings.TrimSpace(c.Query("planId")); pid != "" {
		id, err := strconv.Atoi(pid)
		if err != nil {
			renv.Error(c, http.StatusBadRequest, "无效的 planId")
			return
		}
		planID = id
	}
	stats, err := BuildContributionStats(planID, strings.TrimSpace(c.Query("rootName")))
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, stats)
}

// ── 参数解析 ────────────────────────────────────────────────────

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
