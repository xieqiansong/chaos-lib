package proxy

import (
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
	"sort"

	"gorm.io/gorm/clause"
)

type Bookmark struct {
	ID        string `gorm:"primaryKey"`
	ParentID  string ``
	Title     string ``
	URL       string ``
	IsFolder  bool   ``
	SortIndex int    ``
	DateAdded int64  ``
}

type FrequentBookmark struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	VisitCount    int     `json:"count"`
	LastVisitTime float64 `json:"last"`
}

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

func GetFrequentBookmarks(c *web.Context) {
	q := pagination.Parse(c)
	search := c.Query("search")

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

	renv.Success(c, pagination.New(items, total, q))
}

func GetBookmarkTree(c *web.Context) {
	var flat []Bookmark
	if err := config.GetDB().Order("sort_index ASC").Find(&flat).Error; err != nil {
		renv.Error(c, 500, "查询书签失败: "+err.Error())
		return
	}

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

	var roots []*BookmarkNode
	for _, node := range nodeMap {
		if parent, ok := nodeMap[node.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}

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
