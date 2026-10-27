package plugin

import (
	"fmt"
	"log/slog"

	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

var regs []Registration

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

func All() []Registration { return regs }

func Build() ([]Registration, error) {
	byID := make(map[string]Registration, len(regs))
	for _, r := range regs {
		byID[r.Meta.ID] = r
	}

	for _, r := range regs {
		if !r.Meta.IsEnabled() {
			continue
		}
		for _, dep := range r.Meta.Dependencies {
			if _, ok := byID[dep]; !ok {
				return nil, fmt.Errorf("插件 %q 依赖未注册的插件 %q", r.Meta.ID, dep)
			}
		}
	}

	indeg := make(map[string]int)
	adj := make(map[string][]string)
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
	var queue []string
	for id, d := range indeg {
		if d == 0 {
			queue = append(queue, id)
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
		return nil, fmt.Errorf("检测到插件循环依赖，涉及: %v", inCycle)
	}

	out := make([]Registration, 0, len(ordered))
	for _, id := range ordered {
		out = append(out, byID[id])
	}
	slog.Info("插件依赖校验通过", "order", ordered)
	return out, nil
}

func ModelsOf(rs []Registration) []any {
	var models []any
	for _, r := range rs {
		models = append(models, r.Models...)
	}
	return models
}

func TasksOf(rs []Registration) []scheduler.TaskSpec {
	var tasks []scheduler.TaskSpec
	for _, r := range rs {
		tasks = append(tasks, r.Tasks...)
	}
	return tasks
}

func StartAll(rs []Registration) error {
	for _, r := range rs {
		if r.Start == nil {
			continue
		}
		if err := r.Start(); err != nil {
			return fmt.Errorf("插件 %q 启动失败: %w", r.Meta.ID, err)
		}
		slog.Info("插件启动完成", "id", r.Meta.ID)
	}
	return nil
}

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
