// Package modules 是本应用启用的业务模块清单（副作用导入）。
//
// 每个模块在自己的包内实现 Register 并在 init 中登记到 internal/routehub，
// 因此在本文件中「列出某个包」就等同于「启用该模块的接口」。
// 装配层（internal/app）只需导入本包，router.go 内不再出现任何模块名。
//
// 注意：把某个包移出本清单会导致其路由不再挂载，请确保这是有意为之。
package modules

import (
	_ "chaos-go/internal/apilog"
	_ "chaos-go/internal/cronjob"
	_ "chaos-go/internal/datacache"
	_ "chaos-go/internal/dbmonitor"
	_ "chaos-go/internal/envvar"
	_ "chaos-go/internal/filelink"
	_ "chaos-go/internal/mqttsync"
	_ "chaos-go/internal/notify"
	_ "chaos-go/internal/portfwd"
	_ "chaos-go/internal/project"
	_ "chaos-go/internal/proxy"
	_ "chaos-go/internal/quickedit"
	_ "chaos-go/internal/sdk"
	_ "chaos-go/internal/standarddata"
	_ "chaos-go/internal/systemjob"
	_ "chaos-go/internal/taskplan"
)
