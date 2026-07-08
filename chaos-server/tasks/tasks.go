package tasks

import (
	"log"
	"sync"
	"time"
)

// TaskFunc 后台任务执行函数
type TaskFunc func()

// RegisteredTask 一个已注册的后台周期任务
type RegisteredTask struct {
	Name     string
	Interval time.Duration
	Func     TaskFunc
	Enabled  func() bool // 若返回 false 则跳过启动；nil 表示始终启用
}

var registeredTasks []RegisteredTask

// Register 注册一个后台周期任务（通常在各任务文件的 init() 中调用）
// enabledFn 为可选的开关函数，返回 false 时该任务在 Start 时被跳过
func Register(name string, interval time.Duration, fn TaskFunc, enabledFn ...func() bool) {
	var enabled func() bool
	if len(enabledFn) > 0 {
		enabled = enabledFn[0]
	}
	registeredTasks = append(registeredTasks, RegisteredTask{
		Name:     name,
		Interval: interval,
		Func:     fn,
		Enabled:  enabled,
	})
}

// Start 启动所有已注册且启用的后台任务（在 main.go 中调用一次）
// 每任务一个独立的 goroutine，自带互斥锁，防止上一次还未结束时重复执行
func Start() {
	for _, t := range registeredTasks {
		if t.Enabled != nil && !t.Enabled() {
			log.Printf("⏰ 跳过后台任务 [%s]（已在配置中关闭）", t.Name)
			continue
		}
		task := t
		log.Printf("⏰ 启动后台任务 [%s]，周期 %v", task.Name, task.Interval)

		var mu sync.Mutex

		go func() {
			ticker := time.NewTicker(task.Interval)
			defer ticker.Stop()

			// 启动时先执行一次，然后按周期执行
			safeRun(&mu, task.Name, task.Func)

			for range ticker.C {
				safeRun(&mu, task.Name, task.Func)
			}
		}()
	}
}

// safeRun 互斥地执行一次任务，记录 panic
func safeRun(mu *sync.Mutex, name string, fn TaskFunc) {
	if !mu.TryLock() {
		log.Printf("⏰ 后台任务 [%s] 跳过（上一次还未结束）\n", name)
		return
	}
	defer mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ 后台任务 [%s] panic: %v", name, r)
		}
	}()

	// log.Printf("⏰ 后台任务 [%s] 执行中...\n", name)
	fn()
}
