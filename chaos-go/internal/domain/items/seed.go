package items

import (
	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

// Seed 初始化默认分类与位置（幂等：仅当表为空时写入）。
// 由 cmd/server/main.go 的 app.Run 在自动迁移之后显式调用（init 不得播种）。
func Seed() error {
	db := config.GetDB()
	if db == nil {
		return nil
	}
	seedCategories(db)
	seedLocations(db)
	return nil
}

func seedCategories(db *gorm.DB) {
	var cnt int64
	db.Model(&Category{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	cs := []Category{
		{ID: 1, ParentID: 0, Name: "数码", Icon: strPtr("device"), SortOrder: 1},
		{ID: 2, ParentID: 1, Name: "手机", SortOrder: 1},
		{ID: 3, ParentID: 1, Name: "电脑", SortOrder: 2},
		{ID: 4, ParentID: 1, Name: "外设", SortOrder: 3},
		{ID: 5, ParentID: 0, Name: "生活用品", Icon: strPtr("home"), SortOrder: 2},
		{ID: 6, ParentID: 5, Name: "日用品", SortOrder: 1},
		{ID: 7, ParentID: 0, Name: "服饰", Icon: strPtr("shirt"), SortOrder: 3},
		{ID: 8, ParentID: 0, Name: "交通工具", Icon: strPtr("bike"), SortOrder: 4},
	}
	db.Create(&cs)
}

func seedLocations(db *gorm.DB) {
	var cnt int64
	db.Model(&Location{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	ls := []Location{
		{ID: 1, ParentID: 0, Name: "家", SortOrder: 1},
		{ID: 2, ParentID: 1, Name: "书房", SortOrder: 1},
		{ID: 3, ParentID: 2, Name: "书桌抽屉", SortOrder: 1},
		{ID: 4, ParentID: 0, Name: "办公室", SortOrder: 2},
		{ID: 5, ParentID: 0, Name: "随身携带", SortOrder: 3},
	}
	db.Create(&ls)
}
