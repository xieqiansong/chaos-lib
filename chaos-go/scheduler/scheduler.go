package scheduler

import (
	"log/slog"
	"sync"
	"time"
)

type TaskFunc func()

type RegisteredTask struct {
	Name     string
	Interval time.Duration
	Func     TaskFunc
	Enabled  func() bool
}

var registeredTasks []RegisteredTask

func Register(name string, interval time.Duration, fn TaskFunc, enabledFn ...func() bool) {
	var enabled func() bool
	if len(enabledFn) > 0 {
		enabled = enabledFn[0]
	}
	registeredTasks = append(registeredTasks, RegisteredTask{
		Name: name, Interval: interval, Func: fn, Enabled: enabled,
	})
}

func Start() {
	for _, t := range registeredTasks {
		if t.Enabled != nil && !t.Enabled() {
			slog.Info("跳过后台任务", "name", t.Name, "reason", "已在配置中关闭")
			continue
		}
		task := t
		slog.Info("启动后台任务", "name", task.Name, "interval", task.Interval)
		var mu sync.Mutex
		go func() {
			ticker := time.NewTicker(task.Interval)
			defer ticker.Stop()
			safeRun(&mu, task.Name, task.Func)
			for range ticker.C {
				safeRun(&mu, task.Name, task.Func)
			}
		}()
	}
}

func safeRun(mu *sync.Mutex, name string, fn TaskFunc) {
	if !mu.TryLock() {
		slog.Info("后台任务跳过", "name", name, "reason", "上一次还未结束")
		return
	}
	defer mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("后台任务 panic", "name", name, "panic", r)
		}
	}()
	fn()
}
