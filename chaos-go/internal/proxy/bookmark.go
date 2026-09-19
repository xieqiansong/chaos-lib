package proxy

import (
	"chaos-go/config"
	"chaos-go/internal/pagination"

	"github.com/gin-gonic/gin"
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

// ── Handlers ──────────────────────────────────────────────────────

// SaveBookmarks 批量 upsert 书签扁平快照（由扩展备份调用）。
func SaveBookmarks(c *gin.Context) {
	var items []Bookmark
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(items) == 0 {
		c.JSON(400, gin.H{"error": "数组不能为空"})
		return
	}
	res := config.GetDB().Clauses(clause.OnConflict{UpdateAll: true}).Create(&items)
	if err := res.Error; err != nil {
		c.JSON(500, gin.H{"error": "数据库写入失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "批量保存成功", "count": len(items), "saved_items": items})
}

// GetFrequentBookmarks 返回「常用书签」：书签 ∪ 历史访问次数，按访问频率降序，统一分页（默认 20）。
func GetFrequentBookmarks(c *gin.Context) {
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
		c.JSON(500, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	// 响应统一为 { items, total, page, size }
	c.JSON(200, pagination.New(items, total, q))
}
