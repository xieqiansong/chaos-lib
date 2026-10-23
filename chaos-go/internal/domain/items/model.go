package items

import (
	"time"

	"chaos-go/internal/framework/crud"
)

// 物品类型
const (
	ItemTypeDurable    = 1 // 耐用品
	ItemTypeConsumable = 2 // 消耗品
)

// 物品状态
const (
	StatusInUse     = "in_use"    // 在用
	StatusIdle      = "idle"      // 闲置
	StatusLent      = "lent"      // 借出
	StatusRepairing = "repairing" // 维修
	StatusBroken    = "broken"    // 损坏
	StatusDisposed  = "disposed"  // 已处置
)

// 流水类型
const (
	RecordPurchase  = "purchase"
	RecordConsume   = "consume"
	RecordReplenish = "replenish"
	RecordLend      = "lend"
	RecordReturn    = "return"
	RecordRepair    = "repair"
	RecordMove      = "move"
	RecordDispose   = "dispose"
	RecordInventory = "inventory"
	RecordInstall   = "install"
	RecordRemove    = "remove"
)

// Category 分类字典（支持父子级）。
type Category struct {
	crud.BaseModel
	ParentID  int     `json:"ParentID"`
	Name      string  `json:"Name"`
	Icon      *string `json:"Icon"`
	SortOrder int     `json:"SortOrder"`
}

func (Category) TableName() string { return "item_category" }

// Location 存放位置字典（支持父子级）。
type Location struct {
	crud.BaseModel
	ParentID  int    `json:"ParentID"`
	Name      string `json:"Name"`
	SortOrder int    `json:"SortOrder"`
}

func (Location) TableName() string { return "item_location" }

// Item 物品主表：耐用品与消耗品统一存储，靠 ItemType 区分。
type Item struct {
	crud.BaseModel
	Name              string     `json:"Name"`
	CategoryID        *int       `json:"CategoryID"`
	LocationID        *int       `json:"LocationID"`
	ItemType          int        `json:"ItemType" gorm:"default:1"`
	Brand             *string    `json:"Brand"`
	Model             *string    `json:"Model"`
	Spec              *string    `json:"Spec"`
	Color             *string    `json:"Color"`
	Quantity          float64    `json:"Quantity" gorm:"default:1"`
	Unit              string     `json:"Unit" gorm:"default:'个'"`
	MinQuantity       *float64   `json:"MinQuantity"`
	Status            string     `json:"Status" gorm:"default:'in_use'"`
	SerialNo          *string    `json:"SerialNo" gorm:"index"`
	PurchaseDate      *time.Time `json:"PurchaseDate"`
	PurchasePrice     *float64   `json:"PurchasePrice"`
	PurchaseChannel   *string    `json:"PurchaseChannel"`
	WarrantyEnd       *time.Time `json:"WarrantyEnd"`
	ExpireDate        *time.Time `json:"ExpireDate"`
	LastReplenishDate *time.Time `json:"LastReplenishDate"`
	LastReplenishQty  *float64   `json:"LastReplenishQty"`
	Tags              *string    `json:"Tags"`
	CoverImage        *string    `json:"CoverImage"`
	Remark            *string    `json:"Remark"`
}

func (Item) TableName() string { return "items" }

// ItemRecord 变动流水：所有"发生过的事"都记这里，是库存真相。
type ItemRecord struct {
	crud.BaseModel
	ItemID     int       `json:"ItemID"`
	RecordType string    `json:"RecordType"`
	Quantity   *float64  `json:"Quantity"`
	Amount     *float64  `json:"Amount"`
	RecordDate time.Time `json:"RecordDate"`
	Note       *string   `json:"Note"`
}

func (ItemRecord) TableName() string { return "item_records" }

// ItemAssembly 装配关系：配件作为独立 item 装入主机。
type ItemAssembly struct {
	crud.BaseModel
	HostID      int        `json:"HostID"`
	ComponentID int        `json:"ComponentID"`
	Slot        *string    `json:"Slot"`
	Quantity    float64    `json:"Quantity" gorm:"default:1"`
	InstalledAt time.Time  `json:"InstalledAt"`
	RemovedAt   *time.Time `json:"RemovedAt"`
	InstallNote *string    `json:"InstallNote"`
	RemoveNote  *string    `json:"RemoveNote"`
}

func (ItemAssembly) TableName() string { return "item_assemblies" }

// strPtr 便捷构造 *string。
func strPtr(s string) *string { return &s }
