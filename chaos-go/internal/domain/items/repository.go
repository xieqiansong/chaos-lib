package items

import (
	"errors"
	"time"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

// 领域错误
var (
	ErrItemNotFound     = errors.New("items: 物品不存在")
	ErrCategoryNotFound = errors.New("items: 分类不存在")
	ErrLocationNotFound = errors.New("items: 位置不存在")
	ErrAssemblyNotFound = errors.New("items: 装配关系不存在")
	ErrSelfAssembly     = errors.New("items: 配件不能与主机相同")
	ErrInsufficient     = errors.New("items: 库存不足")
	ErrDBUnavailable    = errors.New("items: 数据库不可用")
)

func db() *gorm.DB { return config.GetDB() }

// WithinTx 在事务中执行 fn；fn 返回错误则整体回滚。
func WithinTx(fn func(tx *gorm.DB) error) error {
	d := db()
	if d == nil {
		return ErrDBUnavailable
	}
	return d.Transaction(fn)
}

// ── 读取（repository 是包内唯一数据访问出口）──────────────────────

func GetItem(tx *gorm.DB, id int) (*Item, error) {
	var it Item
	if err := tx.First(&it, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrItemNotFound
		}
		return nil, err
	}
	return &it, nil
}

func UpdateItemFields(tx *gorm.DB, id int, fields map[string]any) error {
	return tx.Model(&Item{}).Where("id = ?", id).Updates(fields).Error
}

func CreateItem(tx *gorm.DB, it *Item) error            { return tx.Create(it).Error }
func CreateRecord(tx *gorm.DB, r *ItemRecord) error     { return tx.Create(r).Error }
func CreateAssembly(tx *gorm.DB, a *ItemAssembly) error { return tx.Create(a).Error }

// CurrentAssembly 取配件当前在位的装配关系（removed_at 为 NULL）。
func CurrentAssembly(tx *gorm.DB, componentID int, hostID *int) (*ItemAssembly, error) {
	var a ItemAssembly
	q := tx.Where("component_id = ? AND removed_at IS NULL", componentID)
	if hostID != nil {
		q = q.Where("host_id = ?", *hostID)
	}
	if err := q.First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssemblyNotFound
		}
		return nil, err
	}
	return &a, nil
}

// CurrentAssembliesByHost 取主机当前所有在位配件。
func CurrentAssembliesByHost(tx *gorm.DB, hostID int) ([]ItemAssembly, error) {
	var list []ItemAssembly
	if err := tx.Where("host_id = ? AND removed_at IS NULL", hostID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ── 派生查询（只读，供 handler 直接调用）──────────────────────────

// ComponentView 主机当前配置中的单个配件视图。
type ComponentView struct {
	ComponentID int
	Name        string
	Brand       *string
	Model       *string
	Spec        *string
	SerialNo    *string
	Slot        *string
	Quantity    float64
	InstalledAt time.Time
}

// HostConfig 一台主机当前完整配置（仅在位配件）。
func HostConfig(hostID int) ([]ComponentView, error) {
	d := db()
	if d == nil {
		return nil, ErrDBUnavailable
	}
	var rows []ComponentView
	if err := d.Model(&ItemAssembly{}).
		Select("item_assembly.component_id, i.name, i.brand, i.model, i.spec, i.serial_no, item_assembly.slot, item_assembly.quantity, item_assembly.installed_at").
		Joins("JOIN item i ON i.id = item_assembly.component_id").
		Where("item_assembly.host_id = ? AND item_assembly.removed_at IS NULL AND i.is_deleted = 0", hostID).
		Order("item_assembly.slot").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// AssemblyHistory 配件的流转历史视图。
type AssemblyHistory struct {
	HostID      int
	HostName    string
	Slot        *string
	InstalledAt time.Time
	RemovedAt   *time.Time
	State       string
}

// ComponentHistory 某个配件的流转历史（联表不触发软删插件，需显式写 is_deleted = 0）。
func ComponentHistory(componentID int) ([]AssemblyHistory, error) {
	d := db()
	if d == nil {
		return nil, ErrDBUnavailable
	}
	var rows []AssemblyHistory
	sql := `SELECT a.host_id, h.name AS host_name, a.slot, a.installed_at, a.removed_at,
		CASE WHEN a.removed_at IS NULL THEN '在位' ELSE '已拆' END AS state
		FROM item_assembly a JOIN item h ON h.id = a.host_id
		WHERE a.component_id = ? AND h.is_deleted = 0
		ORDER BY a.installed_at DESC`
	if err := d.Raw(sql, componentID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CategoryStat 按分类统计在册物品与总价值（不含已处置）。
type CategoryStat struct {
	CategoryID   int
	CategoryName *string
	Count        int
	TotalValue   float64
}

func CategoryStats() ([]CategoryStat, error) {
	d := db()
	if d == nil {
		return nil, ErrDBUnavailable
	}
	var rows []CategoryStat
	if err := d.Model(&Item{}).
		Select("i.category_id, d.label AS category_name, COUNT(*) AS count, COALESCE(SUM(i.purchase_price * i.quantity), 0) AS total_value").
		Joins("LEFT JOIN dict_data d ON d.id = i.category_id AND d.type_code = ? AND d.is_deleted = 0", "item_category").
		Where("i.is_deleted = 0 AND i.status <> ?", StatusDisposed).
		Group("i.category_id, d.label").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// LowStock 消耗品库存告急列表。
func LowStock() ([]Item, error) {
	d := db()
	if d == nil {
		return nil, ErrDBUnavailable
	}
	var list []Item
	if err := d.Where("item_type = ? AND is_deleted = 0 AND min_quantity IS NOT NULL AND quantity <= min_quantity", ItemTypeConsumable).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// WarrantySoon 保修 N 天内到期提醒。
func WarrantySoon(days int) ([]Item, error) {
	d := db()
	if d == nil {
		return nil, ErrDBUnavailable
	}
	today := time.Now()
	until := today.AddDate(0, 0, days)
	var list []Item
	if err := d.Where("is_deleted = 0 AND warranty_end IS NOT NULL AND warranty_end >= ? AND warranty_end <= ?", today, until).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ReplenishCycle 消耗品补货周期预测。
type ReplenishCycle struct {
	ItemID          int
	ItemName        string
	Cycles          int
	AvgCycleDays    float64
	LastBuy         time.Time
	NextBuyEstimate *time.Time
}

// ReplenishForecast 基于 replenish 流水预测下次补货时间（窗口函数改为 Go 计算，方言无关）。
func ReplenishForecast() ([]ReplenishCycle, error) {
	d := db()
	if d == nil {
		return nil, ErrDBUnavailable
	}
	var recs []ItemRecord
	if err := d.Where("record_type = ? AND is_deleted = 0", RecordReplenish).
		Order("item_id, record_date").Find(&recs).Error; err != nil {
		return nil, err
	}
	type acc struct{ dates []time.Time }
	m := map[int]*acc{}
	order := []int{}
	for _, r := range recs {
		a, ok := m[r.ItemID]
		if !ok {
			a = &acc{}
			m[r.ItemID] = a
			order = append(order, r.ItemID)
		}
		a.dates = append(a.dates, r.RecordDate)
	}
	out := make([]ReplenishCycle, 0, len(order))
	for _, id := range order {
		a := m[id]
		if len(a.dates) < 2 {
			continue
		}
		var sum float64
		n := 0
		for i := 1; i < len(a.dates); i++ {
			sum += a.dates[i].Sub(a.dates[i-1]).Hours() / 24
			n++
		}
		avg := sum / float64(n)
		last := a.dates[len(a.dates)-1]
		next := last.AddDate(0, 0, int(avg))
		out = append(out, ReplenishCycle{
			ItemID:          id,
			Cycles:          n,
			AvgCycleDays:    avg,
			LastBuy:         last,
			NextBuyEstimate: &next,
		})
	}
	if len(out) > 0 {
		ids := make([]int, 0, len(out))
		for _, o := range out {
			ids = append(ids, o.ItemID)
		}
		var items []Item
		d.Where("id IN ?", ids).Find(&items)
		nameMap := make(map[int]string, len(items))
		for _, it := range items {
			nameMap[it.ID] = it.Name
		}
		for i := range out {
			out[i].ItemName = nameMap[out[i].ItemID]
		}
	}
	return out, nil
}
