package note

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

// 路径穿越防护是整套模块的安全底线，必须守住。
func TestResolveSafePath(t *testing.T) {
	// 用真实绝对路径作 root（Windows 下 filepath.Join 产出反斜杠，与生产一致）。
	root := t.TempDir()
	cases := []struct {
		rel     string
		wantErr bool
	}{
		{"", false},
		{"a/b.md", false},
		{"a/../b.md", true},   // 含 .. 一律拒绝（真实 relPath 永不含 ..，安全优先）
		{"../escape.md", true}, // 跳出根目录
		{"a/../../escape.md", true},
		{"/etc/passwd", true}, // 绝对路径输入
		{"a/./b.md", false},   // 规范化
		{"\x00bad", true},     // 非法字符
	}
	for _, c := range cases {
		_, err := resolveSafePath(root, c.rel)
		if c.wantErr {
			if !errors.Is(err, ErrInvalidPath) && !errors.Is(err, ErrPathEscapesVault) {
				t.Errorf("resolveSafePath(%q) 期望错误，得到 %v", c.rel, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveSafePath(%q) 意外错误 %v", c.rel, err)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	withFM := []byte("---\ntitle: 真实标题\n---\n# 这是标题吗\n正文")
	if got := ExtractTitle(withFM, "兜底"); got != "真实标题" {
		t.Errorf("front matter 标题解析失败：%q", got)
	}

	noFM := []byte("# 一级标题\n正文")
	if got := ExtractTitle(noFM, "兜底"); got != "一级标题" {
		t.Errorf("一级标题解析失败：%q", got)
	}

	// 代码块内的 # 不应被当作标题
	inCode := []byte("```\n# 这是代码\n```\n正文")
	if got := ExtractTitle(inCode, "兜底名"); got != "兜底名" {
		t.Errorf("代码块内 # 不应成为标题：%q", got)
	}
}

func TestPlainText(t *testing.T) {
	md := []byte("# 标题\n\n这是 **加粗** 和 [链接](http://x.com) 与 `代码`。\n<!-- 注释 -->")
	got := PlainText(md)
	for _, bad := range []string{"**", "http://x.com", "`代码`", "<!--", " -->"} {
		if strings.Contains(got, bad) {
			t.Errorf("PlainText 未剥离 %q：%q", bad, got)
		}
	}
	if !strings.Contains(got, "标题") || !strings.Contains(got, "加粗") {
		t.Errorf("PlainText 丢失正文：%q", got)
	}
	if len(got) > 65536 {
		t.Errorf("PlainText 未截断：%d", len(got))
	}
}

// buildTree 必须正确嵌套任意深度的目录。早期实现只补齐一层父目录，
// 深度 ≥3 的笔记其祖父目录缺失、挂接时命中孤立分支，整棵深层子树被拍平到根级
//（目录树里表现为出现重名目录）。此测试锁死该回归。
func TestBuildTreeNested(t *testing.T) {
	notes := []Note{
		// 4 层。90-Sources 下没有任何直属 md，祖先目录从未作为 ParentRel 出现过——
		// 这正是「纯目录容器」塌陷的真实场景。
		{ID: 1, RelPath: "90-Sources/牛客面经八股题目/C++ STL/STL.md", ParentRel: "90-Sources/牛客面经八股题目/C++ STL", Name: "STL.md", Title: "STL 笔记"},
		// 3 层。
		{ID: 2, RelPath: "10-技术/前端/React.md", ParentRel: "10-技术/前端", Name: "React.md"},
		// 2 层：让 10-技术 同时拥有直属 md 与子目录。
		{ID: 3, RelPath: "10-技术/学习.md", ParentRel: "10-技术", Name: "学习.md", Title: "学习计划"},
		// 根级文件。
		{ID: 4, RelPath: "README.md", Name: "README.md"},
	}

	roots := buildTree(notes)

	// 关键断言：根节点只能是 10-技术、90-Sources、README.md。
	// 只要有任何一个深层目录被拍平，这个计数就会变大。
	if len(roots) != 3 {
		t.Fatalf("根节点应为 3 个，实际 %d：%v", len(roots), nodeNames(roots))
	}
	if roots[0].Name != "10-技术" || roots[1].Name != "90-Sources" {
		t.Fatalf("根级目录内容/顺序错误：%v", nodeNames(roots))
	}
	if roots[2].Name != "README.md" || !roots[2].IsLeaf {
		t.Fatalf("根级叶子笔记标记错误：%q isLeaf=%v", roots[2].Name, roots[2].IsLeaf)
	}

	// 90-Sources 无直属 md，其三层子树必须完整嵌套其中。
	src := roots[1]
	if len(src.Children) != 1 {
		t.Fatalf("90-Sources 应只含 1 个子目录，实际 %d：%v", len(src.Children), nodeNames(src.Children))
	}
	mid := src.Children[0]
	if mid.Name != "牛客面经八股题目" || mid.IsLeaf {
		t.Fatalf("二级目录错误：%q isLeaf=%v", mid.Name, mid.IsLeaf)
	}
	deep := mid.Children
	if len(deep) != 1 || deep[0].Name != "C++ STL" || deep[0].IsLeaf {
		t.Fatalf("三级目录错误：%v", nodeNames(deep))
	}

	// 叶子笔记必须保持叶子：早期实现在挂接时强制 node.IsLeaf=false，会把它错标成目录。
	leaf := deep[0].Children
	if len(leaf) != 1 || leaf[0].Name != "STL.md" || !leaf[0].IsLeaf {
		t.Fatalf("叶子笔记标记错误：%v", nodeNames(leaf))
	}
	if len(leaf[0].Children) != 0 {
		t.Fatalf("叶子笔记不应有子节点，实际 %d", len(leaf[0].Children))
	}

	// 叶子须携带 ID / Title，前端点击即可直达编辑器（无需再查一次列表）。
	if leaf[0].ID != 1 || leaf[0].Title != "STL 笔记" {
		t.Fatalf("叶子应携带 ID / Title，实际 id=%d title=%q", leaf[0].ID, leaf[0].Title)
	}
	// 目录节点不得携带，否则 JSON 里会多出无意义的零值字段。
	if src.ID != 0 || src.Title != "" {
		t.Fatalf("目录节点不应携带 ID / Title，实际 id=%d title=%q", src.ID, src.Title)
	}

	// 10-技术 同时含直属 md 与子目录：排序上目录优先于文件。
	tech := roots[0]
	if len(tech.Children) != 2 {
		t.Fatalf("10-技术 应含 2 个子节点，实际 %d：%v", len(tech.Children), nodeNames(tech.Children))
	}
	if tech.Children[0].Name != "前端" || tech.Children[0].IsLeaf {
		t.Fatalf("子目录应排在前且标记为目录：%q isLeaf=%v", tech.Children[0].Name, tech.Children[0].IsLeaf)
	}
	if tech.Children[1].Name != "学习.md" || !tech.Children[1].IsLeaf {
		t.Fatalf("直属笔记标记错误：%q isLeaf=%v", tech.Children[1].Name, tech.Children[1].IsLeaf)
	}
}

// nodeNames 收集节点名，便于断言失败时输出可读信息。
func nodeNames(nodes []*TreeNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}

// v1ListRouter 复刻 crud.RegisterActions 中 V1ListHandler 的包装方式：
// 先 MetaToQuery 把信封 meta 桥接成 query，再交给自定义列表 handler。
func v1ListRouter(capture *ListFilter) *web.Router {
	
	r := web.NewRouter()
	g := r.Group("/api/v1/notes")
	g.Use(envelope.Middleware())
	g.POST("/list", func(c *web.Context) {
		envelope.MetaToQuery(c)
		*capture = filterFromRequest(c, pagination.Parse(c))
		renv.Success(c, map[string]any{"ok": true})
	})
	return r
}

// 前端 DataTable 的 search 经 useRestApi 统一收进 meta.like，而 MetaToQuery 不会展开
// 这个 map（只压成一个 like=map[…] 查询串）。早期 listNotes 只读 c.Query，目录过滤恒失效，
// 表现为树里点笔记永远匹配不到、右侧不打开。此测试锁死该修复。
func TestFilterFromRequestMetaLike(t *testing.T) {
	var got ListFilter
	r := v1ListRouter(&got)

	payload := envelope.Request{
		Action: "notes.list",
		Data:   json.RawMessage(`{}`),
		Meta: json.RawMessage(
			`{"page":1,"pageSize":50,"sort":"updated_at","order":"desc",` +
				`"like":{"dir":"10-技术/AI/提示词","q":"面试","starred":true,"missing":false}}`),
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes/list", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}

	if got.Dir != "10-技术/AI/提示词" {
		t.Errorf("dir 期望从 meta.like 取回，实际 %q（为空即目录过滤失效）", got.Dir)
	}
	if got.Query != "面试" {
		t.Errorf("q 期望从 meta.like 取回，实际 %q", got.Query)
	}
	if !got.Starred {
		t.Errorf("starred 期望为 true，实际 %v", got.Starred)
	}
	if got.Missing {
		t.Errorf("missing 期望为 false，实际 %v", got.Missing)
	}
	if got.PageSize != 50 || got.Offset != 0 {
		t.Errorf("分页期望 pageSize=50 offset=0，实际 pageSize=%d offset=%d", got.PageSize, got.Offset)
	}
}

// 存量 RESTful（GET /notes?dir=…）必须保持原行为：没有 meta.like 时回落到 query。
func TestFilterFromRequestQueryFallback(t *testing.T) {
	
	var got ListFilter
	r := web.NewRouter()
	r.GET("/notes", func(c *web.Context) {
		got = filterFromRequest(c, pagination.Parse(c))
		renv.Success(c, map[string]any{"ok": true})
	})

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/notes?dir=30-职业&q=简历&page_size=20", nil))

	if got.Dir != "30-职业" {
		t.Errorf("存量 dir 回落失败，实际 %q", got.Dir)
	}
	if got.Query != "简历" {
		t.Errorf("存量 q 回落失败，实际 %q", got.Query)
	}
	if got.PageSize != 20 {
		t.Errorf("存量 page_size 期望 20，实际 %d", got.PageSize)
	}
}

// 根目录（dir 为空）是合法取值：前端空串不传 like.dir，此时必须落到根目录而非报错。
func TestFilterFromRequestRootDir(t *testing.T) {
	var got ListFilter
	r := v1ListRouter(&got)

	payload := envelope.Request{
		Action: "notes.list",
		Data:   json.RawMessage(`{}`),
		// 前端对空字符串过滤后不写入 like，故此处没有 dir 字段。
		Meta: json.RawMessage(`{"page":1,"pageSize":50,"like":{"starred":false,"missing":false}}`),
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes/list", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if got.Dir != "" {
		t.Errorf("缺省 dir 应为空（根目录），实际 %q", got.Dir)
	}
	if got.Starred || got.Missing {
		t.Errorf("布尔筛选应为 false，starred=%v missing=%v", got.Starred, got.Missing)
	}
}
