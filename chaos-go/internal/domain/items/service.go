package items

import (
	"time"

	"gorm.io/gorm"
)

// 归一化：缺省记录日期用当前时间。
func normalizeDate(t *time.Time) {
	if t == nil {
		return
	}
	if t.IsZero() {
		*t = time.Now()
	}
}

// Purchase 购入物品 + purchase 流水，返回新建的物品。
func Purchase(req PurchaseRequest) (*Item, error) {
	it := req.Item
	it.ID = 0
	it.IsDeleted = 0
	d := req.RecordDate
	normalizeDate(&d)
	req.RecordDate = d
	var created Item
	err := WithinTx(func(tx *gorm.DB) error {
		if err := CreateItem(tx, &it); err != nil {
			return err
		}
		rec := ItemRecord{ItemID: it.ID, RecordType: RecordPurchase, Amount: it.PurchasePrice, RecordDate: req.RecordDate, Note: req.Note}
		if err := CreateRecord(tx, &rec); err != nil {
			return err
		}
		created = it
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// Replenish 消耗品补货：先清旧库存（可选），再进新货并记 replenish 流水。
func Replenish(req ReplenishRequest) (*Item, error) {
	normalizeDate(&req.RecordDate)
	var updated *Item
	err := WithinTx(func(tx *gorm.DB) error {
		item, err := GetItem(tx, req.ItemID)
		if err != nil {
			return err
		}
		consumeOld := req.ConsumeOld == nil || *req.ConsumeOld
		if consumeOld && item.Quantity > 0 {
			old := item.Quantity
			if err := CreateRecord(tx, &ItemRecord{ItemID: item.ID, RecordType: RecordConsume, Quantity: &old, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
				return err
			}
		}
		if err := UpdateItemFields(tx, item.ID, map[string]any{
			"quantity":            req.Qty,
			"last_replenish_date": req.RecordDate,
			"last_replenish_qty":  req.Qty,
		}); err != nil {
			return err
		}
		if err := CreateRecord(tx, &ItemRecord{ItemID: item.ID, RecordType: RecordReplenish, Quantity: &req.Qty, Amount: req.Amount, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
			return err
		}
		u, err := GetItem(tx, item.ID)
		if err != nil {
			return err
		}
		updated = u
		return nil
	})
	return updated, err
}

// Consume 消耗指定数量（库存不足报错）。
func Consume(req ConsumeRequest) (*Item, error) {
	normalizeDate(&req.RecordDate)
	var updated *Item
	err := WithinTx(func(tx *gorm.DB) error {
		item, err := GetItem(tx, req.ItemID)
		if err != nil {
			return err
		}
		if item.Quantity < req.Qty {
			return ErrInsufficient
		}
		newQty := item.Quantity - req.Qty
		if err := CreateRecord(tx, &ItemRecord{ItemID: item.ID, RecordType: RecordConsume, Quantity: &req.Qty, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
			return err
		}
		if err := UpdateItemFields(tx, item.ID, map[string]any{"quantity": newQty}); err != nil {
			return err
		}
		u, err := GetItem(tx, item.ID)
		if err != nil {
			return err
		}
		updated = u
		return nil
	})
	return updated, err
}

// Install 装配配件到主机：建装配关系 + 配件位置跟随主机 + install 流水。
func Install(req InstallRequest) error {
	if req.HostID == req.ComponentID {
		return ErrSelfAssembly
	}
	normalizeDate(&req.InstalledAt)
	return WithinTx(func(tx *gorm.DB) error {
		host, err := GetItem(tx, req.HostID)
		if err != nil {
			return err
		}
		if _, err := GetItem(tx, req.ComponentID); err != nil {
			return err
		}
		asm := ItemAssembly{HostID: req.HostID, ComponentID: req.ComponentID, Slot: req.Slot, Quantity: req.Qty, InstalledAt: req.InstalledAt, InstallNote: req.Note}
		if err := CreateAssembly(tx, &asm); err != nil {
			return err
		}
		if err := UpdateItemFields(tx, req.ComponentID, map[string]any{"location_id": host.LocationID, "status": StatusInUse}); err != nil {
			return err
		}
		if err := CreateRecord(tx, &ItemRecord{ItemID: req.ComponentID, RecordType: RecordInstall, RecordDate: req.InstalledAt, Note: req.Note}); err != nil {
			return err
		}
		return nil
	})
}

// Remove 拆下配件：置 removed_at + 配件转闲置 + remove 流水。
func Remove(req RemoveRequest) error {
	normalizeDate(&req.RemovedAt)
	return WithinTx(func(tx *gorm.DB) error {
		if _, err := GetItem(tx, req.ComponentID); err != nil {
			return err
		}
		asm, err := CurrentAssembly(tx, req.ComponentID, req.HostID)
		if err != nil {
			return err
		}
		if err := tx.Model(&ItemAssembly{}).Where("id = ?", asm.ID).Update("removed_at", req.RemovedAt).Error; err != nil {
			return err
		}
		if err := UpdateItemFields(tx, req.ComponentID, map[string]any{"status": StatusIdle}); err != nil {
			return err
		}
		if err := CreateRecord(tx, &ItemRecord{ItemID: req.ComponentID, RecordType: RecordRemove, RecordDate: req.RemovedAt, Note: req.Note}); err != nil {
			return err
		}
		return nil
	})
}

// Lend 借出：状态转 lent + lend 流水。
func Lend(req IDRequest) error {
	return statusWithRecord(req, StatusLent, RecordLend)
}

// ReturnItem 归还：状态转 in_use + return 流水。
func ReturnItem(req IDRequest) error {
	return statusWithRecord(req, StatusInUse, RecordReturn)
}

// Repair 维修：状态转 repairing + repair 流水（可记费用）。
func Repair(req RepairRequest) error {
	normalizeDate(&req.RecordDate)
	return WithinTx(func(tx *gorm.DB) error {
		if _, err := GetItem(tx, req.ItemID); err != nil {
			return err
		}
		if err := UpdateItemFields(tx, req.ItemID, map[string]any{"status": StatusRepairing}); err != nil {
			return err
		}
		if err := CreateRecord(tx, &ItemRecord{ItemID: req.ItemID, RecordType: RecordRepair, Amount: req.Amount, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
			return err
		}
		return nil
	})
}

// Dispose 处置：自动拆下在位配件，主机转 disposed + dispose 流水。
func Dispose(req IDRequest) error {
	normalizeDate(&req.RecordDate)
	return WithinTx(func(tx *gorm.DB) error {
		item, err := GetItem(tx, req.ItemID)
		if err != nil {
			return err
		}
		asms, err := CurrentAssembliesByHost(tx, item.ID)
		if err != nil {
			return err
		}
		for _, a := range asms {
			if err := tx.Model(&ItemAssembly{}).Where("id = ?", a.ID).Update("removed_at", req.RecordDate).Error; err != nil {
				return err
			}
			if err := UpdateItemFields(tx, a.ComponentID, map[string]any{"status": StatusIdle}); err != nil {
				return err
			}
			if err := CreateRecord(tx, &ItemRecord{ItemID: a.ComponentID, RecordType: RecordRemove, RecordDate: req.RecordDate, Note: strPtr("主机处置，自动拆下")}); err != nil {
				return err
			}
		}
		if err := UpdateItemFields(tx, item.ID, map[string]any{"status": StatusDisposed}); err != nil {
			return err
		}
		if err := CreateRecord(tx, &ItemRecord{ItemID: item.ID, RecordType: RecordDispose, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
			return err
		}
		return nil
	})
}

// Inventory 盘点修正：按实际数量调整并记差额（正=账面多）。
func Inventory(req InventoryRequest) (*Item, error) {
	normalizeDate(&req.RecordDate)
	var updated *Item
	err := WithinTx(func(tx *gorm.DB) error {
		item, err := GetItem(tx, req.ItemID)
		if err != nil {
			return err
		}
		diff := item.Quantity - req.ActualQty
		if err := CreateRecord(tx, &ItemRecord{ItemID: item.ID, RecordType: RecordInventory, Quantity: &diff, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
			return err
		}
		if err := UpdateItemFields(tx, item.ID, map[string]any{"quantity": req.ActualQty}); err != nil {
			return err
		}
		u, err := GetItem(tx, item.ID)
		if err != nil {
			return err
		}
		updated = u
		return nil
	})
	return updated, err
}

// statusWithRecord 通用「改状态 + 写流水」事务。
func statusWithRecord(req IDRequest, status, recType string) error {
	normalizeDate(&req.RecordDate)
	return WithinTx(func(tx *gorm.DB) error {
		if _, err := GetItem(tx, req.ItemID); err != nil {
			return err
		}
		if err := UpdateItemFields(tx, req.ItemID, map[string]any{"status": status}); err != nil {
			return err
		}
		if err := CreateRecord(tx, &ItemRecord{ItemID: req.ItemID, RecordType: recType, RecordDate: req.RecordDate, Note: req.Note}); err != nil {
			return err
		}
		return nil
	})
}
