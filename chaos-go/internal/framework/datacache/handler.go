package datacache

import (
	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "datacache",
			Name:         "数据缓存",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&DataCache{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}

var dataCacheOpts = crud.Opts[DataCache]{

	Searchable: []string{"category", "key", "data_type"},
	Sortable:   []string{"id", "value_len", "created_at"},
}

func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[DataCache](rg, "data-caches", dataCacheOpts, nil)
}
