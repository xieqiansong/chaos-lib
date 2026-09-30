package crud

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/testkit"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// widget 测试模型：与业务表同构（嵌入 BaseModel + 业务列）。
// 所有读写都落在 testkit 提供的临时 sqlite 上，绝不触碰开发/生产库。
type widget struct {
	BaseModel
	Name   string `gorm:"size:64" json:"Name"`
	Kind   string `gorm:"size:32" json:"Kind"`
	Sort   int    `json:"Sort"`
	Status bool   `json:"Status"`
}

func (widget) TableName() string { return "widgets" }

// widgetDTO 用于验证 ToResponse 回调：额外派生一个大写字段。
type widgetDTO struct {
	ID    int    `json:"ID"`
	Name  string `json:"Name"`
	Upper string `json:"Upper"`
}

func toWidgetDTOs(rows []*widget) any {
	out := make([]widgetDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, widgetDTO{ID: r.ID, Name: r.Name, Upper: strings.ToUpper(r.Name)})
	}
	return out
}

// setupWidgets 建一个临时库 + 只挂了 widgets 资源的路由。
func setupWidgets(t *testing.T, opts Opts[widget]) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := testkit.NewTestDB(t, &widget{})
	r := testkit.Router(func(api *gin.RouterGroup) { Register[widget](api, "widgets", opts) })
	return r, db
}

// pageOf 解析统一分页载荷。
type pageOf struct {
	List       []json.RawMessage `json:"list"`
	Pagination struct {
		Page       int   `json:"page"`
		PageSize   int   `json:"page_size"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"total_pages"`
	} `json:"pagination"`
}

func parsePage(t *testing.T, data json.RawMessage) pageOf {
	t.Helper()
	var p pageOf
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("分页载荷解析失败: %v\ndata=%s", err, data)
	}
	return p
}

// namesOf 取出列表里每条记录的 Name。
func namesOf(t *testing.T, p pageOf) []string {
	t.Helper()
	names := make([]string, 0, len(p.List))
	for _, raw := range p.List {
		var row map[string]any
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatalf("列表项解析失败: %v", err)
		}
		name, _ := row["Name"].(string)
		names = append(names, name)
	}
	return names
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func countAlive(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&widget{}).Where("is_deleted = ?", false).Count(&n).Error; err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	return n
}

// ── 生命周期list/get/create/patch/delete ──

func TestCRUDLifecycle(t *testing.T) {
	r, db := setupWidgets(t, Opts[widget]{})

	created := testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha","Kind":"tool","Sort":3}`))
	var first map[string]any
	if err := json.Unmarshal(created, &first); err != nil {
		t.Fatalf("创建响应解析失败: %v", err)
	}
	id, _ := first["ID"].(float64)
	if id != 1 {
		t.Fatalf("创建返回的 ID = %v, want 1", first["ID"])
	}
	if first["Name"] != "alpha" {
		t.Fatalf("创建返回的 Name = %v", first["Name"])
	}

	data := testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets", ""))
	p := parsePage(t, data)
	if p.Pagination.Total != 1 || p.Pagination.Page != 1 || p.Pagination.PageSize != 20 || p.Pagination.TotalPages != 1 {
		t.Fatalf("分页元信息不符：%+v", p.Pagination)
	}
	if got := namesOf(t, p); !equalStrings(got, []string{"alpha"}) {
		t.Fatalf("列表内容不符：%v", got)
	}

	data = testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets/1", ""))
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("详情响应解析失败: %v", err)
	}
	if got["ID"] != float64(1) || got["Name"] != "alpha" {
		t.Fatalf("详情内容不符：%v", got)
	}
	if _, leaked := got["IsDeleted"]; leaked {
		t.Fatal("详情不应泄漏 IsDeleted 字段")
	}

	data = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPatch, "/api/widgets/1", `{"Name":"alpha2"}`))
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("更新响应解析失败: %v", err)
	}
	if got["Name"] != "alpha2" {
		t.Fatalf("更新返回的 Name = %v", got["Name"])
	}

	var persisted widget
	if err := db.First(&persisted, 1).Error; err != nil {
		t.Fatalf("更新未落库: %v", err)
	}
	if persisted.Name != "alpha2" {
		t.Fatalf("落库 Name = %q, want alpha2", persisted.Name)
	}

	data = testkit.RequireOK(t, testkit.Do(t, r, http.MethodDelete, "/api/widgets/1", ""))
	if string(data) != "null" {
		t.Fatalf("删除响应 data 期望 null，实际 %s", data)
	}

	if n := countAlive(t, db); n != 0 {
		t.Fatalf("软删后存活记录数 = %d, want 0", n)
	}
	var trashed widget
	if err := db.Unscoped().First(&trashed, 1).Error; err != nil {
		t.Fatalf("删除应为软删（记录仍存在）: %v", err)
	}
	if !trashed.IsDeleted {
		t.Fatal("软删标记未置位")
	}

	p = parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets", "")))
	if p.Pagination.Total != 0 || len(p.List) != 0 {
		t.Fatalf("软删记录不应出现在列表：%+v", p.Pagination)
	}
	testkit.RequireError(t, testkit.Do(t, r, http.MethodGet, "/api/widgets/1", ""), http.StatusNotFound)
}

func TestCRUDNotFoundAndBadRequest(t *testing.T) {
	r, _ := setupWidgets(t, Opts[widget]{})

	// 先落一条，确保「更新 body 非法」能真正走到 ShouldBindJSON 分支而不是先 404
	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"详情不存在", http.MethodGet, "/api/widgets/404", "", http.StatusNotFound},
		{"更新不存在", http.MethodPatch, "/api/widgets/404", `{"Name":"x"}`, http.StatusNotFound},
		{"删除不存在", http.MethodDelete, "/api/widgets/404", "", http.StatusNotFound},
		{"创建字段类型非法", http.MethodPost, "/api/widgets", `{"Sort":"three"}`, http.StatusBadRequest},
		{"更新不是 JSON", http.MethodPatch, "/api/widgets/1", "{", http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testkit.RequireError(t, testkit.Do(t, r, c.method, c.path, c.body), c.want)
		})
	}
}

// ── 分页 / 搜索 / 排序 ──

func TestCRUDListPagingSearchAndSort(t *testing.T) {
	r, db := setupWidgets(t, Opts[widget]{
		Searchable: []string{"name"},
		Sortable:   []string{"id", "sort"},
	})

	rows := []widget{
		{BaseModel: BaseModel{ID: 1}, Name: "alpha", Kind: "k1", Sort: 3},
		{BaseModel: BaseModel{ID: 2}, Name: "beta", Kind: "k2", Sort: 1},
		{BaseModel: BaseModel{ID: 3}, Name: "gamma", Kind: "k3", Sort: 2},
		// 已被软删：任何列表查询都不应看到
		{BaseModel: BaseModel{ID: 4, IsDeleted: true}, Name: "delta", Kind: "k4", Sort: 4},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}

	t.Run("默认按 id 倒序并过滤软删", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets", "")))
		if p.Pagination.Total != 3 {
			t.Fatalf("total = %d, want 3（软删记录须不计）", p.Pagination.Total)
		}
		if got := namesOf(t, p); !equalStrings(got, []string{"gamma", "beta", "alpha"}) {
			t.Fatalf("默认排序不符：%v", got)
		}
	})

	t.Run("page_size 生效且算出总页数", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets?page=2&page_size=2", "")))
		if p.Pagination.Total != 3 || p.Pagination.TotalPages != 2 || p.Pagination.Page != 2 {
			t.Fatalf("分页元信息不符：%+v", p.Pagination)
		}
		if got := namesOf(t, p); !equalStrings(got, []string{"alpha"}) {
			t.Fatalf("第二页内容不符：%v", got)
		}
	})

	t.Run("可搜字段做模糊匹配", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets?name=bet", "")))
		if got := namesOf(t, p); !equalStrings(got, []string{"beta"}) {
			t.Fatalf("搜索结果不符：%v", got)
		}
	})

	t.Run("白名单内升序", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets?sort=sort&order=asc", "")))
		if got := namesOf(t, p); !equalStrings(got, []string{"beta", "gamma", "alpha"}) {
			t.Fatalf("sort 升序不符：%v", got)
		}
	})

	t.Run("白名单内降序", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets?sort=sort&order=desc", "")))
		if got := namesOf(t, p); !equalStrings(got, []string{"alpha", "gamma", "beta"}) {
			t.Fatalf("sort 降序不符：%v", got)
		}
	})

	t.Run("白名单外排序被忽略", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets?sort=kind&order=asc", "")))
		if got := namesOf(t, p); !equalStrings(got, []string{"gamma", "beta", "alpha"}) {
			t.Fatalf("非白名单排序应回落到 id DESC，实际 %v", got)
		}
	})

	t.Run("排序字段注入被白名单挡下", func(t *testing.T) {
		p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets?sort=id%3B+DROP+TABLE+widgets", "")))
		if got := namesOf(t, p); !equalStrings(got, []string{"gamma", "beta", "alpha"}) {
			t.Fatalf("注入串应被忽略，实际 %v", got)
		}
		if n := countAlive(t, db); n != 3 {
			t.Fatalf("注入后记录数 = %d, want 3（表不应被改写）", n)
		}
	})
}

func TestCRUDListHandlerOverride(t *testing.T) {
	r, _ := setupWidgets(t, Opts[widget]{
		// 业务包需要「列表带外部副作用」时走这条路径（如项目管理要合并未认领目录）
		ListHandler: func(c *gin.Context) {
			renv.Success(c, pagination.New([]string{"custom"}, 1, pagination.Query{Page: 1, PageSize: 20}))
		},
	})

	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))

	p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets", "")))
	if len(p.List) != 1 || string(p.List[0]) != `"custom"` {
		t.Fatalf("应走自定义 ListHandler，实际 list=%s", p.List)
	}
}

// ── 字段保护 ──

func TestCRUDProtectedFieldsCannotBePatched(t *testing.T) {
	r, db := setupWidgets(t, Opts[widget]{Protected: []string{"status"}})

	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha","Status":true}`))

	// 驼峰写法与列名写法都要被挡：两者经 camelToSnake 归一化后等价
	for _, key := range []string{"Status", "status"} {
		t.Run(key+"不可改写", func(t *testing.T) {
			_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPatch, "/api/widgets/1", `{"`+key+`":false}`))

			var row widget
			if err := db.First(&row, 1).Error; err != nil {
				t.Fatalf("读取记录失败: %v", err)
			}
			if !row.Status {
				t.Fatalf("%s 被通用 PATCH 改写了，应只能走专属路由", key)
			}
		})
	}

	t.Run("受保护字段不连带阻止其它字段", func(t *testing.T) {
		_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPatch, "/api/widgets/1", `{"Name":"alpha2","Status":false}`))

		var row widget
		if err := db.First(&row, 1).Error; err != nil {
			t.Fatalf("读取记录失败: %v", err)
		}
		if row.Name != "alpha2" {
			t.Fatalf("Name 未更新，实际 %q", row.Name)
		}
		if !row.Status {
			t.Fatal("同一请求里受保护字段仍应被剔除")
		}
	})
}

func TestCRUDBaseFieldsCannotBePatched(t *testing.T) {
	r, db := setupWidgets(t, Opts[widget]{})

	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))

	before := widget{}
	if err := db.First(&before, 1).Error; err != nil {
		t.Fatalf("读取记录失败: %v", err)
	}

	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPatch, "/api/widgets/1",
		`{"ID":99,"CreatedAt":"2000-01-01T00:00:00Z","UpdatedAt":"2000-01-01T00:00:00Z","IsDeleted":true}`))

	var after widget
	if err := db.First(&after, 1).Error; err != nil {
		t.Fatalf("基字段被改写后记录消失: %v", err)
	}
	if after.ID != before.ID {
		t.Fatalf("ID 不应可改：%d -> %d", before.ID, after.ID)
	}
	if after.IsDeleted {
		t.Fatal("is_deleted 不应可经 PATCH 改写")
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at 不应可改：%v -> %v", before.CreatedAt, after.CreatedAt)
	}

	// 仍然可见：说明没被“悄悄软删”
	if n := countAlive(t, db); n != 1 {
		t.Fatalf("存活记录数 = %d, want 1", n)
	}
}

// ── ToResponse 映射 ──

func TestCRUDToResponseAppliesToListAndSingle(t *testing.T) {
	r, _ := setupWidgets(t, Opts[widget]{ToResponse: toWidgetDTOs})

	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))
	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"beta"}`))

	p := parsePage(t, testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets", "")))
	if len(p.List) != 2 {
		t.Fatalf("列表条数 = %d, want 2", len(p.List))
	}
	var last widgetDTO
	if err := json.Unmarshal(p.List[0], &last); err != nil {
		t.Fatalf("列表项解析失败: %v", err)
	}
	// 默认 id DESC：先出现 beta
	if last.Name != "beta" || last.Upper != "BETA" {
		t.Fatalf("列表项未经 ToResponse 映射：%+v", last)
	}

	data := testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets/1", ""))
	var one widgetDTO
	if err := json.Unmarshal(data, &one); err != nil {
		t.Fatalf("详情解析失败: %v", err)
	}
	if one.ID != 1 || one.Name != "alpha" || one.Upper != "ALPHA" {
		t.Fatalf("详情未经 ToResponse 映射（或取错元素）：%+v", one)
	}
}

// ── 钩子与事务回滚 ──

func TestCRUDHookFailuresRollback(t *testing.T) {
	t.Run("BeforeCreate 失败不落库", func(t *testing.T) {
		r, db := setupWidgets(t, Opts[widget]{BeforeCreate: func(row *widget) error {
			if strings.TrimSpace(row.Name) == "" {
				return errors.New("名称不能为空")
			}
			return nil
		}})

		testkit.RequireError(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"  "}`), http.StatusBadRequest)
		if n := countAlive(t, db); n != 0 {
			t.Fatalf("校验被拒却仍有 %d 条记录", n)
		}

		_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))
		if n := countAlive(t, db); n != 1 {
			t.Fatalf("合法数据应创建成功，实际 %d 条", n)
		}
	})

	t.Run("AfterCreate 失败整笔回滚", func(t *testing.T) {
		r, db := setupWidgets(t, Opts[widget]{AfterCreate: func(row *widget) error {
			return errors.New("副作用失败")
		}})

		testkit.RequireError(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`), http.StatusBadRequest)
		if n := countAlive(t, db); n != 0 {
			t.Fatalf("钩子失败应回滚，实际仍有 %d 条", n)
		}
	})

	t.Run("AfterUpdate 失败保留旧值", func(t *testing.T) {
		r, db := setupWidgets(t, Opts[widget]{AfterUpdate: func(row *widget) error {
			return errors.New("同步失败")
		}})
		_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))

		testkit.RequireError(t, testkit.Do(t, r, http.MethodPatch, "/api/widgets/1", `{"Name":"alpha2"}`), http.StatusBadRequest)

		var row widget
		if err := db.First(&row, 1).Error; err != nil {
			t.Fatalf("读取记录失败: %v", err)
		}
		if row.Name != "alpha" {
			t.Fatalf("钩子失败应回滚到旧值，实际 %q", row.Name)
		}
	})

	t.Run("AfterDelete 失败不软删", func(t *testing.T) {
		r, db := setupWidgets(t, Opts[widget]{AfterDelete: func(row *widget) error {
			return errors.New("清理失败")
		}})
		_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha"}`))

		testkit.RequireError(t, testkit.Do(t, r, http.MethodDelete, "/api/widgets/1", ""), http.StatusBadRequest)

		var row widget
		if err := db.First(&row, 1).Error; err != nil {
			t.Fatalf("记录不应被删除: %v", err)
		}
		if row.IsDeleted {
			t.Fatal("钩子失败不应留下软删标记")
		}
		if n := countAlive(t, db); n != 1 {
			t.Fatalf("存活记录数 = %d, want 1", n)
		}
	})
}

func TestCRUDHooksObserveRow(t *testing.T) {
	var createdName string
	r, _ := setupWidgets(t, Opts[widget]{
		BeforeCreate: func(row *widget) error {
			row.Kind = strings.ToLower(row.Kind) // 规范化：与业务包的前置校验同构
			return nil
		},
		AfterCreate: func(row *widget) error {
			createdName = row.Name
			if row.ID == 0 {
				t.Fatal("AfterCreate 应拿到已分配主键的记录")
			}
			return nil
		},
	})

	_ = testkit.RequireOK(t, testkit.Do(t, r, http.MethodPost, "/api/widgets", `{"Name":"alpha","Kind":"TOOL"}`))
	if createdName != "alpha" {
		t.Fatalf("AfterCreate 未收到同一条记录，got %q", createdName)
	}

	data := testkit.RequireOK(t, testkit.Do(t, r, http.MethodGet, "/api/widgets/1", ""))
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatalf("详情解析失败: %v", err)
	}
	if row["Kind"] != "tool" {
		t.Fatalf("BeforeCreate 的规范化未生效，Kind = %v", row["Kind"])
	}
}

func TestCamelToSnake(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"status":        "status",
		"Status":        "status",
		"IsDeleted":     "is_deleted",
		"sortOrder":     "sort_order",
		"already_snake": "already_snake",
		"ID":            "i_d", // 基字段另有按名剔除清单兜底，故不必依赖本映射
	}
	for in, want := range cases {
		if got := camelToSnake(in); got != want {
			t.Fatalf("camelToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}
