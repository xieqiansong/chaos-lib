package proxy

import (
	"chaos-go/config"
	"chaos-go/internal/pagination"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// ── 模型 ──────────────────────────────────────────────────────────

type BrowserHistory struct {
	ID            string  `gorm:"primaryKey"`
	LastVisitTime float64 ``
	Title         string  ``
	TypeCount     int     ``
	Url           string  ``
	VisitCount    int     ``
}

type BrowserHistoryVisit struct {
	ID               string  `gorm:"primaryKey"`
	HistoryID        string  ``
	IsLocal          bool    ``
	ReferringVisitID string  ``
	Transition       string  ``
	VisitID          string  ``
	VisitTime        float64 ``
}

// ── Handlers ──────────────────────────────────────────────────────

func GetBrowserHistories(c *gin.Context) {
	q := pagination.Parse(c)
	search := c.Query("search")

	// 统一分页：page/size 由 pagination 模块解析（size 强制约束在 [1, MaxSize]）
	base := config.GetDB().Model(&BrowserHistory{})
	if search != "" {
		like := "%" + search + "%"
		base = base.Where("title ILIKE ? OR url ILIKE ?", like, like)
	}
	base = base.Order("last_visit_time DESC")

	var histories []BrowserHistory
	total, err := pagination.Paginate(base, &histories, q)
	if err != nil {
		c.JSON(500, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	// 响应统一为 { items, total, page, size }
	c.JSON(200, pagination.New(histories, total, q))
}

func SaveBrowserHistory(c *gin.Context) {
	var histories []BrowserHistory
	if err := c.ShouldBindJSON(&histories); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(histories) == 0 {
		c.JSON(400, gin.H{"error": "数组不能为空"})
		return
	}
	for i := range histories {
		if histories[i].Url != "" {
			hash := sha256.Sum256([]byte(histories[i].Url))
			histories[i].ID = hex.EncodeToString(hash[:])
		}
	}
	res := config.GetDB().Debug().Clauses(clause.OnConflict{UpdateAll: true}).Create(&histories)
	if err := res.Error; err != nil {
		c.JSON(500, gin.H{"error": "数据库写入失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "批量保存成功", "count": len(histories), "saved_items": histories})
}

func SaveBrowserHistoryVisits(c *gin.Context) {
	var visits []BrowserHistoryVisit
	if err := c.ShouldBindJSON(&visits); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(visits) == 0 {
		c.JSON(400, gin.H{"error": "数组不能为空"})
		return
	}
	for i := range visits {
		url := visits[i].ID
		idHash := sha256.Sum256([]byte(fmt.Sprintf("%s_%f", url, visits[i].VisitTime)))
		visits[i].ID = hex.EncodeToString(idHash[:])
		urlHash := sha256.Sum256([]byte(url))
		visits[i].HistoryID = hex.EncodeToString(urlHash[:])
	}
	seen := make(map[string]bool)
	uniqueList := make([]BrowserHistoryVisit, 0)
	for _, v := range visits {
		if !seen[v.ID] {
			seen[v.ID] = true
			uniqueList = append(uniqueList, v)
		}
	}
	res := config.GetDB().Clauses(clause.OnConflict{UpdateAll: true}).Create(&uniqueList)
	if err := res.Error; err != nil {
		c.JSON(500, gin.H{"error": "数据库写入失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "批量保存成功", "count": len(visits), "saved_items": visits})
}
