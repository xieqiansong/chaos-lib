package taskplan

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

func GetTaskDailyStats(c *web.Context) {
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

func GetTaskActiveStats(c *web.Context) {
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

func GetTaskContributionStats(c *web.Context) {
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

func parseDaysParam(c *web.Context, def int) (int, error) {
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

func parseRange(c *web.Context, today time.Time) (start, end time.Time, err error) {
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

func parseDateParam(c *web.Context, name string, def time.Time) (time.Time, error) {
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
