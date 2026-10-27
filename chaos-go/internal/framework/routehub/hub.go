package routehub

import (
	"fmt"

	"chaos-go/internal/framework/web"
)

type MountFunc func(rg *web.RouterGroup)

type module struct {
	name  string
	mount MountFunc
}

var modulesV1 []module

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

func MountAllV1(rg *web.RouterGroup) []string {
	names := make([]string, 0, len(modulesV1))
	for _, m := range modulesV1 {
		m.mount(rg)
		names = append(names, m.name)
	}
	return names
}

func Registered() []string {
	names := make([]string, 0, len(modulesV1))
	for _, m := range modulesV1 {
		names = append(names, m.name)
	}
	return names
}
