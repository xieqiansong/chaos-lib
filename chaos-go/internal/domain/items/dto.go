package items

import "time"

// PurchaseRequest 购入一件物品（耐用品 / 消耗品），并记一笔 purchase 流水。
type PurchaseRequest struct {
	Item       Item      `json:"item"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

// ReplenishRequest 消耗品补货：可选先清掉上一批，再进新货。
type ReplenishRequest struct {
	ItemID     int       `json:"itemId"`
	Qty        float64   `json:"qty"`
	Amount     *float64  `json:"amount"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
	ConsumeOld *bool     `json:"consumeOld"` // 默认 true：先把旧库存记为消耗
}

// ConsumeRequest 消耗指定数量。
type ConsumeRequest struct {
	ItemID     int       `json:"itemId"`
	Qty        float64   `json:"qty"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

// InstallRequest 把配件装入主机。
type InstallRequest struct {
	HostID      int       `json:"hostId"`
	ComponentID int       `json:"componentId"`
	Slot        *string   `json:"slot"`
	Qty         float64   `json:"qty"`
	InstalledAt time.Time `json:"installedAt"`
	Note        *string   `json:"note"`
}

// RemoveRequest 拆下配件。
type RemoveRequest struct {
	ComponentID int       `json:"componentId"`
	HostID      *int      `json:"hostId"` // 可选：指定从哪台主机拆
	RemovedAt   time.Time `json:"removedAt"`
	Note        *string   `json:"note"`
}

// IDRequest 仅需物品 ID 的状态类动作（借出 / 归还 / 处置）。
type IDRequest struct {
	ItemID     int       `json:"itemId"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

// RepairRequest 维修（顺带记录费用）。
type RepairRequest struct {
	ItemID     int       `json:"itemId"`
	Amount     *float64  `json:"amount"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}

// InventoryRequest 盘点修正：按实际数量调整并记差额。
type InventoryRequest struct {
	ItemID     int       `json:"itemId"`
	ActualQty  float64   `json:"actualQty"`
	RecordDate time.Time `json:"recordDate"`
	Note       *string   `json:"note"`
}
