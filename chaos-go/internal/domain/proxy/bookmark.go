package proxy

import (
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"
	"sort"

	"chaos-go/internal/framework/web"
	"gorm.io/gorm/clause"
)

// ── 模型 ──────────────────────────────────────────────────────────

// Bookmark 是浏览器书签树的扁平化快照，由扩展周期性备份到本表。
// 仅叶子节点带 url；文件夹的 url 为空。常用书签通过 url 关联 browser_histories 的访问次数。
type Bookmark struct {
	ID        string `gorm:"primaryKey"`
	ParentID  string ``
	Title     string ``
	URL       string ``
	IsFolder  bool   ``
	SortIndex int    ``
	DateAdded int64  ``
}

// FrequentBookmark 是「常用书签」接口的响应项：书签基础信息 + 关联的历史访问次数/时间。
type FrequentBookmark struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	VisitCount    int     `json:"count"`
	LastVisitTime float64 `json:"last"`
}

// BookmarkNode 是书签树的嵌套节点，用于前端展示。
// 结构与 Chrome extensions.bookmarks.getTree() 对齐，便于前端复用现有逻辑。
type BookmarkNode struct {
	ID        string          `json:"id"`
	ParentID  string          `json:"parentId,omitempty"`
	Title     string          `json:"title"`
	URL       string          `json:"url,omitempty"`
	IsFolder  bool            `json:"isFolder"`
	SortIndex int             `json:"sortIndex"`
	DateAdded int64           `json:"dateAdded"`
	Children  []*BookmarkNode `json:"children,omitempty"`
}

// ── Handlers ──────────────────────────────────────────────────────

// SaveBookmarks 批量 upsert 书签扁平快照（由扩展备份调用）。
func SaveBookmarks(c *web.Context) {
	var items []Bookmark
	if err := envelope.Bind(c, &items); err != nil {
		renv.Error(c, 400, err.Error())
		return
	}
	if len(items) == 0 {
		renv.Error(c, 400, "数组不能为空")
		return
	}
	res := config.GetDB().Clauses(clause.OnConflict{UpdateAll: true}).Create(&items)
	if err := res.Error; err != nil {
		renv.Error(c, 500, "数据库写入失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}

// GetFrequentBookmarks 返回「常用书签」：书签 ∪ 历史访问次数，按访问频率降序，统一分页（默认 20）。
func GetFrequentBookmarks(c *web.Context) {
	q := pagination.Parse(c)
	search := c.Query("search")

	// 以 bookmarks 为主表，按 url 关联 browser_histories 的访问次数；
	// 同一 url 可能出现在多个文件夹，用 GROUP BY url 去重（取任一 id/title）；
	// 仅保留「带 url 且确有访问记录」的书签，按访问次数（其次最近访问）排序。
	base := config.GetDB().
		Table("bookmarks b").
		Select("MAX(b.id) AS id, MAX(b.title) AS title, b.url AS url, h.visit_count AS visit_count, h.last_visit_time AS last_visit_time").
		Joins("JOIN browser_histories h ON b.url = h.url").
		Where("b.url <> '' AND h.visit_count > 0").
		Group("b.url, h.visit_count, h.last_visit_time")

	if search != "" {
		like := "%" + search + "%"
		base = base.Where("b.title ILIKE ? OR b.url ILIKE ?", like, like)
	}
	base = base.Order("h.visit_count DESC, h.last_visit_time DESC")

	var items []FrequentBookmark
	total, err := pagination.Paginate(base, &items, q)
	if err != nil {
		renv.Error(c, 500, "查询失败: "+err.Error())
		return
	}
	// 响应统一为 { items, total, page, size }
	renv.Success(c, pagination.New(items, total, q))
}

// GetBookmarkTree 返回完整书签树（从扁平 bookmarks 表按 parent_id 组装）。
// 顶层节点的 parent_id 为空串或不在表中时视为根。
func GetBookmarkTree(c *web.Context) {
	var flat []Bookmark
	if err := config.GetDB().Find(&flat).Error; err != nil {
		renv.Error(c, 500, "查询书签失败: "+err.Error())
		return
	}

	// id → 节点 映射，便于按 parent_id 快速定位父节点
	nodeMap := make(map[string]*BookmarkNode, len(flat))
	for i := range flat {
		b := &flat[i]
		nodeMap[b.ID] = &BookmarkNode{
			ID:        b.ID,
			ParentID:  b.ParentID,
			Title:     b.Title,
			URL:       b.URL,
			IsFolder:  b.IsFolder,
			SortIndex: b.SortIndex,
			DateAdded: b.DateAdded,
		}
	}

	// 组装树：子节点挂到父节点的 Children 下；父节点不存在的进入根层
	// Children 用指针切片，避免 map 迭代顺序随机导致拷贝后子节点的嵌套不完整
	var roots []*BookmarkNode
	for _, node := range nodeMap {
		if parent, ok := nodeMap[node.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}

	// ---- 排序 ----
	bySort := func(s []*BookmarkNode) {
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].SortIndex < s[j].SortIndex
		})
	}
	bySort(roots)
	for _, node := range nodeMap {
		if len(node.Children) > 1 {
			bySort(node.Children)
		}
	}

	renv.Success(c, roots)
}
