package plugin

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

// regs 保存全部已登记的插件（按登记顺序）。
var regs []Registration

// Register 登记一个插件。重复 ID 直接 panic（尽早暴露配置错误）。
func Register(r Registration) {
	if r.Meta.ID == "" {
		panic("plugin: 空 ID 的插件登记")
	}
	for _, e := range regs {
		if e.Meta.ID == r.Meta.ID {
			panic(fmt.Sprintf("plugin: 重复登记插件 %q", r.Meta.ID))
		}
	}
	regs = append(regs, r)
}

// All 返回当前已登记的全部插件（未做校验/排序）。
func All() []Registration { return regs }

// Build 做依赖校验与拓扑排序，返回依赖有序的「启用」插件列表。
// 缺失依赖或硬循环依赖会返回 error（装配层据此 fail-fast）。
func Build() ([]Registration, error) {
	byID := make(map[string]Registration, len(regs))
	for _, r := range regs {
		byID[r.Meta.ID] = r
	}

	// 缺失依赖 / 依赖已禁用检测（SoftDeps 不计入硬依赖）。
	// 依赖被禁用时明确报错，避免 Kahn 入度把该边计入后误报成「循环依赖」。
	for _, r := range regs {
		if !r.Meta.IsEnabled() {
			continue
		}
		for _, dep := range r.Meta.Dependencies {
			d, ok := byID[dep]
			if !ok {
				return nil, fmt.Errorf("插件 %q 依赖未注册的插件 %q", r.Meta.ID, dep)
			}
			if !d.Meta.IsEnabled() {
				return nil, fmt.Errorf("插件 %q 依赖的插件 %q 已禁用", r.Meta.ID, dep)
			}
		}
	}

	// Kahn 拓扑排序（仅启用插件参与；边为硬依赖）。
	// 初始队列按登记顺序构建，保证同等优先级下启动顺序稳定、日志可复现。
	indeg := make(map[string]int)
	adj := make(map[string][]string)
	var queue []string
	for _, r := range regs {
		if !r.Meta.IsEnabled() {
			continue
		}
		if _, ok := indeg[r.Meta.ID]; !ok {
			indeg[r.Meta.ID] = 0
		}
		for _, dep := range r.Meta.Dependencies {
			adj[dep] = append(adj[dep], r.Meta.ID)
			indeg[r.Meta.ID]++
		}
	}
	for _, r := range regs {
		if r.Meta.IsEnabled() && indeg[r.Meta.ID] == 0 {
			queue = append(queue, r.Meta.ID)
		}
	}
	ordered := make([]string, 0, len(indeg))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, id)
		for _, nxt := range adj[id] {
			indeg[nxt]--
			if indeg[nxt] == 0 {
				queue = append(queue, nxt)
			}
		}
	}
	if len(ordered) != len(indeg) {
		var inCycle []string
		for id, d := range indeg {
			if d > 0 {
				inCycle = append(inCycle, id)
			}
		}
		sort.Strings(inCycle)
		return nil, fmt.Errorf("检测到插件循环依赖，涉及: %v", inCycle)
	}

	out := make([]Registration, 0, len(ordered))
	for _, id := range ordered {
		out = append(out, byID[id])
	}
	slog.Info("插件依赖校验通过", "order", ordered)
	return out, nil
}

// ModelsOf 汇总所有插件的 GORM 模型，供装配层统一 AutoMigrate。
func ModelsOf(rs []Registration) []any {
	var models []any
	for _, r := range rs {
		models = append(models, r.Models...)
	}
	return models
}

// TasksOf 汇总所有插件的内置任务声明，供装配层交给 scheduler 统一运行。
func TasksOf(rs []Registration) []scheduler.TaskSpec {
	var tasks []scheduler.TaskSpec
	for _, r := range rs {
		tasks = append(tasks, r.Tasks...)
	}
	return tasks
}

// StartAll 按依赖拓扑序启动插件（调用非 nil 的 Start）。
// 单个插件启动失败不阻断其它插件：错误聚合后返回，由装配层决定是否降级继续。
func StartAll(rs []Registration) error {
	var errs []error
	for _, r := range rs {
		if r.Start == nil {
			continue
		}
		if err := safeStart(r); err != nil {
			errs = append(errs, err)
			continue
		}
		slog.Info("插件启动完成", "id", r.Meta.ID)
	}
	return errors.Join(errs...)
}

// safeStart 把插件 Start 的 panic 转成 error，避免一个插件拖垮整个启动流程。
func safeStart(r Registration) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("插件 %q 启动 panic: %v", r.Meta.ID, p)
		}
	}()
	if err := r.Start(); err != nil {
		return fmt.Errorf("插件 %q 启动失败: %w", r.Meta.ID, err)
	}
	return nil
}

// StopAll 逆序停止插件（调用非 nil 的 Stop），用于优雅退出。
func StopAll(rs []Registration) error {
	for i := len(rs) - 1; i >= 0; i-- {
		r := rs[i]
		if r.Stop == nil {
			continue
		}
		if err := r.Stop(); err != nil {
			return fmt.Errorf("插件 %q 停止失败: %w", r.Meta.ID, err)
		}
	}
	return nil
}

// MountAll 把已登记插件的路由直接挂载到传入的分组（通常是 /api/v1）。
// 各插件 RegisterRoutes 内部直接在该分组上建子路由（如 RegisterV1(rg)），
// 不再经 routehub 二次登记，避免「遍历 modulesV1 时追加元素导致漏挂」的问题。
// 路由按前缀隔离，插件间挂载顺序无关紧要。
func MountAll(rg *web.RouterGroup) {
	for _, r := range regs {
		if !r.Meta.IsEnabled() {
			continue
		}
		if r.RegisterRoutes != nil {
			r.RegisterRoutes(rg)
		}
	}
}
