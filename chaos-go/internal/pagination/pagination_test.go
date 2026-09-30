package pagination

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaos-go/internal/testkit"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// paginationItem 测试用表：所有查询都落在临时 sqlite 上，与真实库无关。
type paginationItem struct {
	ID       int `gorm:"primaryKey"`
	Name     string
	ParentID int
}

func ctxWithQuery(query string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/x?"+query, nil)
	return c
}

func TestParseDefaultsAndClamps(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  Query
	}{
		{"缺省值", "", Query{Page: 1, PageSize: 20}},
		{"正常取值", "page=3&page_size=5", Query{Page: 3, PageSize: 5}},
		{"page 非法回落 1", "page=abc", Query{Page: 1, PageSize: 20}},
		{"page 为 0 回落 1", "page=0", Query{Page: 1, PageSize: 20}},
		{"负 page 回落 1", "page=-2", Query{Page: 1, PageSize: 20}},
		{"page_size 非法回落 20", "page_size=abc", Query{Page: 1, PageSize: 20}},
		{"page_size 为 0 回落 20", "page_size=0", Query{Page: 1, PageSize: 20}},
		{"page_size 取到上限", "page_size=200", Query{Page: 1, PageSize: 200}},
		{"page_size 超上限回落 20", "page_size=201", Query{Page: 1, PageSize: 20}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Parse(ctxWithQuery(c.query))
			if got != c.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", c.query, got, c.want)
			}
		})
	}
}

func TestQueryOffset(t *testing.T) {
	cases := []struct {
		q    Query
		want int
	}{
		{Query{Page: 1, PageSize: 20}, 0},
		{Query{Page: 2, PageSize: 20}, 20},
		{Query{Page: 3, PageSize: 10}, 20},
		{Query{Page: 5, PageSize: 200}, 800},
	}
	for _, c := range cases {
		if got := c.q.Offset(); got != c.want {
			t.Fatalf("Query%+v.Offset() = %d, want %d", c.q, got, c.want)
		}
	}
}

func TestScopeAppendsLimitAndOffset(t *testing.T) {
	db := testkit.NewTestDB(t)

	q := Query{Page: 3, PageSize: 10}
	tx := db.Session(&gorm.Session{DryRun: true}).
		Model(&paginationItem{}).
		Scopes(q.Scope).
		Find(&[]paginationItem{})

	sql := strings.ToUpper(tx.Statement.SQL.String())
	if !strings.Contains(sql, "LIMIT") {
		t.Fatalf("Scope 未注入 LIMIT：%s", sql)
	}
	if !strings.Contains(sql, "OFFSET") {
		t.Fatalf("Scope 未注入 OFFSET：%s", sql)
	}
	if !strings.Contains(tx.Statement.SQL.String(), "20") || !strings.Contains(tx.Statement.SQL.String(), "10") {
		t.Fatalf("Scope 参数应含 limit=10 与 offset=20，实际 %s", tx.Statement.SQL.String())
	}
}

func TestNewComputesTotalPages(t *testing.T) {
	cases := []struct {
		name  string
		total int64
		q     Query
		want  int
	}{
		{"无记录", 0, Query{Page: 1, PageSize: 20}, 0},
		{"不足一页", 1, Query{Page: 1, PageSize: 20}, 1},
		{"刚好一页", 20, Query{Page: 1, PageSize: 20}, 1},
		{"多一条多一页", 21, Query{Page: 1, PageSize: 20}, 2},
		{"整除", 200, Query{Page: 1, PageSize: 20}, 10},
		{"页长为 0 时为 0", 5, Query{Page: 1, PageSize: 0}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := New(nil, c.total, c.q)
			if got.Pagination.TotalPages != c.want {
				t.Fatalf("total_pages = %d, want %d", got.Pagination.TotalPages, c.want)
			}
			if got.Pagination.Total != c.total {
				t.Fatalf("total = %d, want %d", got.Pagination.Total, c.total)
			}
			if got.Pagination.Page != c.q.Page || got.Pagination.PageSize != c.q.PageSize {
				t.Fatalf("page/page_size 未透传：%+v", got.Pagination)
			}
		})
	}
}

// TestPaginate 覆盖两个关键点：
//  1. base 未显式带 Model 时，Count 仍能靠 dest 元素类型推出表名（历史 bug：Table not set）；
//  2. Count 之后同一 base 仍可用于 Find（Session 克隆生效）。
func TestPaginateWithoutExplicitModel(t *testing.T) {
	db := testkit.NewTestDB(t, &paginationItem{})

	rows := []paginationItem{
		{ID: 1, Name: "a", ParentID: 7},
		{ID: 2, Name: "b", ParentID: 7},
		{ID: 3, Name: "c", ParentID: 7},
		{ID: 4, Name: "d", ParentID: 8},
		{ID: 5, Name: "e", ParentID: 8},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}

	// 刻意不带 Model，且 Where/Order 与 Count/Fill 共用同一条语句
	base := db.Where("parent_id = ?", 7).Order("id ASC")

	var first []paginationItem
	total, err := Paginate[paginationItem](base, &first, Query{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("Paginate 报错: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(first) != 2 || first[0].ID != 1 || first[1].ID != 2 {
		t.Fatalf("第一页内容不符：%+v", first)
	}

	// 复用同一条 base：验证 Session 克隆确实隔离了 Count 的 SELECT count(*)
	var second []paginationItem
	total, err = Paginate[paginationItem](base, &second, Query{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("第二次 Paginate 报错: %v", err)
	}
	if total != 3 {
		t.Fatalf("第二次 total = %d, want 3", total)
	}
	if len(second) != 1 || second[0].ID != 3 {
		t.Fatalf("第二页内容不符：%+v", second)
	}
}

// TestPaginateRespectsWhereAndOrder 验证过滤/排序真正下推到 SQL。
func TestPaginateRespectsWhereAndOrder(t *testing.T) {
	db := testkit.NewTestDB(t, &paginationItem{})

	rows := []paginationItem{
		{ID: 1, Name: "a", ParentID: 1},
		{ID: 2, Name: "b", ParentID: 1},
		{ID: 3, Name: "c", ParentID: 2},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}

	var got []paginationItem
	total, err := Paginate[paginationItem](
		db.Model(&paginationItem{}).Where("parent_id = ?", 1).Order("id DESC"),
		&got, Query{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Paginate 报错: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Fatalf("排序未按 id DESC：%+v", got)
	}
}
