package items

import (
	"time"

	"chaos-go/internal/framework/crud"
)

const (
	ItemTypeDurable    = 1
	ItemTypeConsumable = 2
)

const (
	StatusInUse     = "in_use"
	StatusIdle      = "idle"
	StatusLent      = "lent"
	StatusRepairing = "repairing"
	StatusBroken    = "broken"
	StatusDisposed  = "disposed"
)

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

type Category struct {
	crud.BaseModel
	ParentID  int     `json:"ParentID"`
	Name      string  `json:"Name"`
	Icon      *string `json:"Icon"`
	SortOrder int     `json:"SortOrder"`
}

func (Category) TableName() string { return "item_category" }

type Location struct {
	crud.BaseModel
	ParentID  int    `json:"ParentID"`
	Name      string `json:"Name"`
	SortOrder int    `json:"SortOrder"`
}

func (Location) TableName() string { return "item_location" }

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

func strPtr(s string) *string { return &s }
