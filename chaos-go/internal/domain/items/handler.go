package items

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

// errRules 领域错误 → HTTP 状态码映射。
var errRules = []httpx.ErrRule{
	{Err: ErrItemNotFound, Status: http.StatusNotFound, Msg: "物品不存在"},
	{Err: ErrCategoryNotFound, Status: http.StatusNotFound, Msg: "分类不存在"},
	{Err: ErrLocationNotFound, Status: http.StatusNotFound, Msg: "位置不存在"},
	{Err: ErrAssemblyNotFound, Status: http.StatusNotFound, Msg: "装配关系不存在"},
	{Err: ErrSelfAssembly, Status: http.StatusBadRequest, Msg: "配件不能与主机相同"},
	{Err: ErrInsufficient, Status: http.StatusBadRequest, Msg: "库存不足"},
	{Err: ErrDBUnavailable, Status: http.StatusInternalServerError, Msg: "数据库不可用"},
}

// RegisterV1 挂载个人物品模块到 /api/v1（POST + Action 风格）。
// 字典表 / 流水 / 装配表走通用 CRUD；物品主表的"业务动作"单独挂载在 items 组下。
func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[Category](rg, "categories", crud.Opts[Category]{
		Searchable: []string{"name"},
		Sortable:   []string{"id", "sort_order", "created_at"},
	}, nil)
	crud.RegisterActions[Location](rg, "locations", crud.Opts[Location]{
		Searchable: []string{"name"},
		Sortable:   []string{"id", "sort_order", "created_at"},
	}, nil)
	crud.RegisterActions[ItemRecord](rg, "item-records", crud.Opts[ItemRecord]{
		Searchable: []string{"note"},
		Sortable:   []string{"id", "record_date", "created_at"},
	}, nil)
	crud.RegisterActions[ItemAssembly](rg, "item-assemblies", crud.Opts[ItemAssembly]{
		Searchable: []string{"slot"},
		Sortable:   []string{"id", "installed_at", "created_at"},
	}, nil)

	g := crud.RegisterActions[Item](rg, "items", crud.Opts[Item]{
		Searchable: []string{"name", "brand", "model", "serial_no", "tags", "remark"},
		Sortable:   []string{"id", "quantity", "warranty_end", "expire_date", "purchase_date", "created_at"},
		Protected:  []string{"Quantity"}, // 数量只能由流水动作改动，禁止通用 update 直接改
	}, nil)

	// 业务动作（事务型，先写流水再更新主表）
	g.POST("/purchase", purchaseAction)
	g.POST("/replenish", replenishAction)
	g.POST("/consume", consumeAction)
	g.POST("/install", installAction)
	g.POST("/remove", removeAction)
	g.POST("/lend", lendAction)
	g.POST("/return", returnAction)
	g.POST("/repair", repairAction)
	g.POST("/dispose", disposeAction)
	g.POST("/inventory", inventoryAction)

	// 派生查询（只读）
	g.POST("/lowStock", lowStockAction)
	g.POST("/warrantySoon", warrantySoonAction)
	g.POST("/categoryStats", categoryStatsAction)
	g.POST("/hostConfig", hostConfigAction)
	g.POST("/componentHistory", componentHistoryAction)
	g.POST("/replenishForecast", replenishForecastAction)
}

func bind[T any](c *web.Context) (*T, bool) {
	var req T
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return nil, false
	}
	return &req, true
}

func purchaseAction(c *web.Context) {
	req, ok := bind[PurchaseRequest](c)
	if !ok {
		return
	}
	it, err := Purchase(*req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, it)
}

func replenishAction(c *web.Context) {
	req, ok := bind[ReplenishRequest](c)
	if !ok {
		return
	}
	it, err := Replenish(*req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, it)
}

func consumeAction(c *web.Context) {
	req, ok := bind[ConsumeRequest](c)
	if !ok {
		return
	}
	it, err := Consume(*req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, it)
}

func installAction(c *web.Context) {
	req, ok := bind[InstallRequest](c)
	if !ok {
		return
	}
	if err := Install(*req); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func removeAction(c *web.Context) {
	req, ok := bind[RemoveRequest](c)
	if !ok {
		return
	}
	if err := Remove(*req); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func lendAction(c *web.Context) {
	req, ok := bind[IDRequest](c)
	if !ok {
		return
	}
	if err := Lend(*req); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func returnAction(c *web.Context) {
	req, ok := bind[IDRequest](c)
	if !ok {
		return
	}
	if err := ReturnItem(*req); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func repairAction(c *web.Context) {
	req, ok := bind[RepairRequest](c)
	if !ok {
		return
	}
	if err := Repair(*req); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func disposeAction(c *web.Context) {
	req, ok := bind[IDRequest](c)
	if !ok {
		return
	}
	if err := Dispose(*req); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func inventoryAction(c *web.Context) {
	req, ok := bind[InventoryRequest](c)
	if !ok {
		return
	}
	it, err := Inventory(*req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, it)
}

func lowStockAction(c *web.Context) {
	rows, err := LowStock()
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, rows)
}

func warrantySoonAction(c *web.Context) {
	var req struct {
		Days int `json:"days"`
	}
	_ = envelope.Bind(c, &req)
	days := req.Days
	if days <= 0 {
		days = 30
	}
	rows, err := WarrantySoon(days)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, rows)
}

func categoryStatsAction(c *web.Context) {
	rows, err := CategoryStats()
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, rows)
}

func hostConfigAction(c *web.Context) {
	var req struct {
		HostID int `json:"hostId"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.HostID == 0 {
		renv.Error(c, http.StatusBadRequest, "缺少 hostId")
		return
	}
	rows, err := HostConfig(req.HostID)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, rows)
}

func componentHistoryAction(c *web.Context) {
	var req struct {
		ComponentID int `json:"componentId"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.ComponentID == 0 {
		renv.Error(c, http.StatusBadRequest, "缺少 componentId")
		return
	}
	rows, err := ComponentHistory(req.ComponentID)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, rows)
}

func replenishForecastAction(c *web.Context) {
	rows, err := ReplenishForecast()
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, rows)
}
