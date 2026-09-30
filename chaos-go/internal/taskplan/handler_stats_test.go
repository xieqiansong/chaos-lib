package taskplan

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ctxWithQuery 构造带 query 的 gin 上下文（不发起真实 HTTP 请求）。
func ctxWithQuery(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tasks/stats?"+query, nil)
	return c
}

func weeksLater(t time.Time, days int) time.Time { return t.AddDate(0, 0, days) }

func TestParseDaysParam(t *testing.T) {
	today := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)

	cases := []struct {
		name    string
		query   string
		want    int
		wantErr bool
	}{
		{"缺省回退", "", 29, false},
		{"合法值", "days=7", 7, false},
		{"上限不校验", "days=3650", 3650, false},
		{"零非法", "days=0", 29, true},
		{"负数非法", "days=-3", 29, true},
		{"非数字非法", "days=abc", 29, true},
		{"小数非法", "days=7.5", 29, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseDaysParam(ctxWithQuery(c.query), 29)
			if c.wantErr && err == nil {
				t.Fatalf("days=%q 应报错", c.query)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("days=%q 不应报错: %v", c.query, err)
			}
			if got != c.want {
				t.Fatalf("parseDaysParam(%q) = %d, want %d", c.query, got, c.want)
			}
		})
	}
	_ = weeksLater(today, 1)
}

func TestParseDateParam(t *testing.T) {
	def := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)

	got, err := parseDateParam(ctxWithQuery("start=2026-03-10"), "start", def)
	if err != nil {
		t.Fatalf("合法日期报错: %v", err)
	}
	if got.Format("2006-01-02") != "2026-03-10" {
		t.Fatalf("解析结果 = %v", got)
	}

	if got, err := parseDateParam(ctxWithQuery(""), "start", def); err != nil || !got.Equal(def) {
		t.Fatalf("缺省应回退默认值：got=%v err=%v", got, err)
	}

	for _, bad := range []string{"start=2026/03/10", "start=2026-13-01", "start=tomorrow"} {
		if _, err := parseDateParam(ctxWithQuery(bad), "start", def); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}
}

func TestParseRange(t *testing.T) {
	today := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local)

	t.Run("缺省为今天-6 到今天+24", func(t *testing.T) {
		start, end, err := parseRange(ctxWithQuery(""), today)
		if err != nil {
			t.Fatalf("缺省范围报错: %v", err)
		}
		if !start.Equal(weeksLater(today, -6)) {
			t.Fatalf("start = %v, want %v", start, weeksLater(today, -6))
		}
		if !end.Equal(weeksLater(today, 24)) {
			t.Fatalf("end = %v, want %v", end, weeksLater(today, 24))
		}
	})

	t.Run("显式区间", func(t *testing.T) {
		start, end, err := parseRange(ctxWithQuery("start=2026-03-01&end=2026-03-10"), today)
		if err != nil {
			t.Fatalf("合法区间报错: %v", err)
		}
		if start.Format("2006-01-02") != "2026-03-01" || end.Format("2006-01-02") != "2026-03-10" {
			t.Fatalf("区间不符：%v ~ %v", start, end)
		}
	})

	t.Run("同一天合法", func(t *testing.T) {
		if _, _, err := parseRange(ctxWithQuery("start=2026-03-10&end=2026-03-10"), today); err != nil {
			t.Fatalf("起止同一天不应报错: %v", err)
		}
	})

	t.Run("结束早于开始被拒", func(t *testing.T) {
		if _, _, err := parseRange(ctxWithQuery("start=2026-03-10&end=2026-03-09"), today); err == nil {
			t.Fatal("倒挂区间应报错")
		}
	})

	t.Run("非法格式向上冒泡", func(t *testing.T) {
		if _, _, err := parseRange(ctxWithQuery("start=2026-3-10"), today); err == nil {
			t.Fatal("非法 start 应报错")
		}
		if _, _, err := parseRange(ctxWithQuery("end=xx"), today); err == nil {
			t.Fatal("非法 end 应报错")
		}
	})
}

func TestBuildContributionSeries(t *testing.T) {
	start := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	today := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local)

	series := buildContributionSeries(7, "每日任务", start, today, map[string]int{
		"2026-03-08": 2,
		"2026-03-10": 3,
		"未知日期":       99, // 区间外的计数不应计入
	})

	if series["id"] != 7 || series["name"] != "每日任务" {
		t.Fatalf("基础字段不符：%+v", series)
	}
	if series["total"] != 5 {
		t.Fatalf("total = %v, want 5", series["total"])
	}

	days, ok := series["days"].([]map[string]interface{})
	if !ok {
		t.Fatalf("days 类型不符：%T", series["days"])
	}
	if len(days) != 3 {
		t.Fatalf("days 长度 = %d, want 3（start ~ today 含端点）", len(days))
	}
	want := []struct {
		date  string
		count int
	}{
		{"2026-03-08", 2},
		{"2026-03-09", 0},
		{"2026-03-10", 3},
	}
	for i, w := range want {
		if days[i]["date"] != w.date || days[i]["count"] != w.count {
			t.Fatalf("第 %d 天不符：%+v, want %s=%d", i, days[i], w.date, w.count)
		}
	}
}

func TestBuildContributionSeriesEmptyRange(t *testing.T) {
	// start 晚于 today 时不产出任何点，也不应 panic
	start := time.Date(2026, 3, 11, 0, 0, 0, 0, time.Local)
	today := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local)

	series := buildContributionSeries(1, "空", start, today, map[string]int{"2026-03-10": 1})
	days := series["days"].([]map[string]interface{})
	if len(days) != 0 {
		t.Fatalf("days 长度 = %d, want 0", len(days))
	}
	if series["total"] != 0 {
		t.Fatalf("total = %v, want 0", series["total"])
	}
}
