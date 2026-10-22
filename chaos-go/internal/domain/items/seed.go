package items

import (
	"encoding/json"
	"fmt"

	"chaos-go/internal/domain/dict"
	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

// Seed 在自动迁移之后由 app.Run 显式调用。
// 职责：把个人物品原有的「物品分类 / 存放位置 / 购入来源」三类枚举收敛到数据字典（dict_data），
//   1) 确保 dict_type 中存在对应类型（item_category / item_location / purchase_source）；
//   2) 若数据字典中该类型为空：优先迁移旧表已有数据（重映射自增 ID 并修正 Item 引用），
//      否则播种默认数据，保证全新库也有开箱即用的枚举；
//   3) 购入来源由 item.purchase_channel 去重得到。
// 全程幂等：已迁移过（dict_data 该类型非空）则跳过，可安全重复调用。
//
// 旧表 item_category / item_location 在 plugin.Models 中已移除（不再由 GORM 托管），
// 仅在本迁移里作为一次性读取源；迁移完成后它们成为无人引用的遗留表，可随时手工删除。
func Seed() error {
	db := config.GetDB()
	if db == nil {
		return nil
	}

	migrateDictType(db, "item_category", "物品分类")
	migrateDictType(db, "item_location", "存放位置")
	migrateDictType(db, "purchase_source", "购入来源")

	migrateCategory(db)
	migrateLocation(db)
	migratePurchaseChannel(db)
	return nil
}

// migrateDictType 幂等地确保某数据字典类型存在。
func migrateDictType(db *gorm.DB, code, name string) {
	var cnt int64
	db.Model(&dict.DictType{}).Where("code = ?", code).Count(&cnt)
	if cnt > 0 {
		return
	}
	db.Create(&dict.DictType{Code: code, Name: name, Status: true, Sort: 0})
}

// dictDataCount 返回某类型下的字典数据项数量。
func dictDataCount(db *gorm.DB, typeCode string) int64 {
	var cnt int64
	db.Model(&dict.DictData{}).Where("type_code = ?", typeCode).Count(&cnt)
	return cnt
}

// migrateCategory 迁移 item_category → dict_data(type=item_category)，并重映射 item.category_id。
func migrateCategory(db *gorm.DB) {
	const typeCode = "item_category"
	if dictDataCount(db, typeCode) > 0 {
		return
	}
	var cats []Category
	if err := db.Table("item_category").Where("is_deleted = 0").Order("id").Find(&cats).Error; err != nil || len(cats) == 0 {
		seedDefaultCategories(db)
		return
	}
	_ = db.Transaction(func(tx *gorm.DB) error {
		oldToNew := make(map[int]int, len(cats))
		for _, c := range cats {
			row := dict.DictData{
				TypeCode: typeCode,
				Label:    c.Name,
				Value:    c.Name,
				ParentID: 0,
				Sort:     c.SortOrder,
				Status:   true,
				Ext:      iconExt(c.Icon),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			oldToNew[c.ID] = row.ID
		}
		for _, c := range cats {
			if c.ParentID != 0 {
				if p, ok := oldToNew[c.ParentID]; ok {
					if err := tx.Model(&dict.DictData{}).Where("id = ?", oldToNew[c.ID]).Update("parent_id", p).Error; err != nil {
						return err
					}
				}
			}
		}
		return remapItemRef(tx, "category_id", oldToNew)
	})
}

// migrateLocation 迁移 item_location → dict_data(type=item_location)，并重映射 item.location_id。
func migrateLocation(db *gorm.DB) {
	const typeCode = "item_location"
	if dictDataCount(db, typeCode) > 0 {
		return
	}
	var locs []Location
	if err := db.Table("item_location").Where("is_deleted = 0").Order("id").Find(&locs).Error; err != nil || len(locs) == 0 {
		seedDefaultLocations(db)
		return
	}
	_ = db.Transaction(func(tx *gorm.DB) error {
		oldToNew := make(map[int]int, len(locs))
		for _, l := range locs {
			row := dict.DictData{
				TypeCode: typeCode,
				Label:    l.Name,
				Value:    l.Name,
				ParentID: 0,
				Sort:     l.SortOrder,
				Status:   true,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			oldToNew[l.ID] = row.ID
		}
		for _, l := range locs {
			if l.ParentID != 0 {
				if p, ok := oldToNew[l.ParentID]; ok {
					if err := tx.Model(&dict.DictData{}).Where("id = ?", oldToNew[l.ID]).Update("parent_id", p).Error; err != nil {
						return err
					}
				}
			}
		}
		return remapItemRef(tx, "location_id", oldToNew)
	})
}

// migratePurchaseChannel 由 item.purchase_channel 去重得到 purchase_source 字典。
// Item.PurchaseChannel 直接存放来源文本（等于字典 Value），无需重映射引用。
func migratePurchaseChannel(db *gorm.DB) {
	const typeCode = "purchase_source"
	if dictDataCount(db, typeCode) > 0 {
		return
	}
	var chs []string
	db.Model(&Item{}).
		Where("purchase_channel IS NOT NULL AND purchase_channel <> ''").
		Distinct("purchase_channel").Pluck("purchase_channel", &chs)
	if len(chs) == 0 {
		seedDefaultSources(db)
		return
	}
	for i, ch := range chs {
		db.Create(&dict.DictData{TypeCode: typeCode, Label: ch, Value: ch, Sort: i, Status: true})
	}
}

// remapItemRef 用单条 CASE 语句把 item 的某列（category_id / location_id）旧 ID 批量改写为新 ID，
// 用单行 CASE 避免「先改 A→B、又把恰好等于 B 的记录误改成 C」的链式错乱。
func remapItemRef(tx *gorm.DB, col string, m map[int]int) error {
	if len(m) == 0 {
		return nil
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	caseSQL := "CASE " + col
	for _, k := range keys {
		caseSQL += fmt.Sprintf(" WHEN %d THEN %d", k, m[k])
	}
	caseSQL += " ELSE " + col + " END"
	return tx.Model(&Item{}).Where(col+" IN ?", keys).Update(col, gorm.Expr(caseSQL)).Error
}

// iconExt 把图标名序列化为 dict_data.Ext 里的 JSON（与前端约定 {"icon":"device"}）。
func iconExt(icon *string) string {
	if icon == nil || *icon == "" {
		return ""
	}
	if b, e := json.Marshal(map[string]string{"icon": *icon}); e == nil {
		return string(b)
	}
	return ""
}

// ── 默认数据（全新库播种）─────────────────────────────────

func seedDefaultCategories(db *gorm.DB) {
	const typeCode = "item_category"
	nodes := []struct {
		name, icon, parent string
	}{
		{"数码", "device", ""},
		{"手机", "", "数码"},
		{"电脑", "", "数码"},
		{"外设", "", "数码"},
		{"生活用品", "home", ""},
		{"日用品", "", "生活用品"},
		{"服饰", "shirt", ""},
		{"交通工具", "bike", ""},
	}
	byName := map[string]int{}
	for _, n := range nodes {
		row := dict.DictData{TypeCode: typeCode, Label: n.name, Value: n.name, Sort: 0, Status: true, Ext: iconExt(&n.icon)}
		db.Create(&row)
		byName[n.name] = row.ID
	}
	for _, n := range nodes {
		if n.parent != "" {
			if pid, ok := byName[n.parent]; ok {
				db.Model(&dict.DictData{}).Where("id = ?", byName[n.name]).Update("parent_id", pid)
			}
		}
	}
}

func seedDefaultLocations(db *gorm.DB) {
	const typeCode = "item_location"
	nodes := []struct {
		name, parent string
	}{
		{"家", ""},
		{"书房", "家"},
		{"书桌抽屉", "书房"},
		{"办公室", ""},
		{"随身携带", ""},
	}
	byName := map[string]int{}
	for _, n := range nodes {
		row := dict.DictData{TypeCode: typeCode, Label: n.name, Value: n.name, Sort: 0, Status: true}
		db.Create(&row)
		byName[n.name] = row.ID
	}
	for _, n := range nodes {
		if n.parent != "" {
			if pid, ok := byName[n.parent]; ok {
				db.Model(&dict.DictData{}).Where("id = ?", byName[n.name]).Update("parent_id", pid)
			}
		}
	}
}

func seedDefaultSources(db *gorm.DB) {
	const typeCode = "purchase_source"
	for i, s := range []string{"淘宝", "京东", "闲鱼", "拼多多", "线下", "其他"} {
		db.Create(&dict.DictData{TypeCode: typeCode, Label: s, Value: s, Sort: i, Status: true})
	}
}
