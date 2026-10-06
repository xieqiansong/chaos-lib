// Package routehub 是路由挂载函数的登记中心。
//
// 各业务模块在自己的包内实现 RegisterV1(rg *gin.RouterGroup)，并在 init 中调用本包的
// RegisterV1 完成登记；装配层（internal/app）只需 blank import 各业务包，
// 再由 router 调用 MountAllV1 统一挂载到 /api/v1。由此：
//   - 路由注册与业务实现同处一个包，新增模块不必改动 router.go；
//   - 依赖方向始终是「业务包 → routehub（仅依赖 gin）」，不会出现 router 与业务包互相引用的环。
//
// 挂载顺序：按登记顺序（即各包 init 的先后，由 Go 的包初始化规则决定，同一份代码稳定复现）。
// 当前各模块路由前缀互不重叠，顺序不影响匹配；若将来出现前缀冲突（如 /xx/:id 与 /xx/new
// 跨模块重叠），可在 MountAllV1 内改为按显式清单排序。
package routehub

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// MountFunc 是路由挂载函数签名，与各业务模块的 RegisterV1(rg *gin.RouterGroup) 一致。
type MountFunc func(rg *gin.RouterGroup)

// module 是一次路由登记：name 用于去重、日志与顺序排查。
type module struct {
	name  string
	mount MountFunc
}

// modulesV1 是「POST + Action」接口的路由登记（挂在 /api/v1 下）。
var modulesV1 []module

// RegisterV1 登记一个「POST + Action」模块，挂在 /api/v1 下，由业务包 init 调用。
// name 需全局唯一。
func RegisterV1(name string, mount MountFunc) {
	if mount == nil {
		panic(fmt.Sprintf("routehub: v1 模块 %q 的挂载函数为 nil", name))
	}
	for _, m := range modulesV1 {
		if m.name == name {
			panic(fmt.Sprintf("routehub: v1 模块 %q 重复登记", name))
		}
	}
	modulesV1 = append(modulesV1, module{name: name, mount: mount})
}

// MountAllV1 按登记顺序把全部 v1 模块挂载到 rg，并返回实际挂载序列（便于日志与排查）。
func MountAllV1(rg *gin.RouterGroup) []string {
	names := make([]string, 0, len(modulesV1))
	for _, m := range modulesV1 {
		m.mount(rg)
		names = append(names, m.name)
	}
	return names
}

// Registered 返回已登记 v1 模块的当前顺序，不触发挂载（用于诊断）。
func Registered() []string {
	names := make([]string, 0, len(modulesV1))
	for _, m := range modulesV1 {
		names = append(names, m.name)
	}
	return names
}
