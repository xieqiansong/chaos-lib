package items

import "time"

type PurchaseRequest struct {
	Item       Item      `json:"item"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

type ReplenishRequest struct {
	ItemID     int       `json:"itemId"`
	Qty        float64   `json:"qty"`
	Amount     *float64  `json:"amount"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
	ConsumeOld *bool     `json:"consumeOld"`
}

type ConsumeRequest struct {
	ItemID     int       `json:"itemId"`
	Qty        float64   `json:"qty"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

type InstallRequest struct {
	HostID      int       `json:"hostId"`
	ComponentID int       `json:"componentId"`
	Slot        *string   `json:"slot"`
	Qty         float64   `json:"qty"`
	InstalledAt time.Time `json:"installedAt"`
	Note        *string   `json:"note"`
}

type RemoveRequest struct {
	ComponentID int       `json:"componentId"`
	HostID      *int      `json:"hostId"`
	RemovedAt   time.Time `json:"removedAt"`
	Note        *string   `json:"note"`
}

type IDRequest struct {
	ItemID     int       `json:"itemId"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

type RepairRequest struct {
	ItemID     int       `json:"itemId"`
	Amount     *float64  `json:"amount"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

type InventoryRequest struct {
	ItemID     int       `json:"itemId"`
	ActualQty  float64   `json:"actualQty"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}
