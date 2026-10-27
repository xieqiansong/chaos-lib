// Package scheduler 是「内置任务」的调度运行时：任务由插件在代码里声明
// （plugin.Registration.Tasks），由本包按 cron 表达式统一运行，并提供最近一次的执行状态。
//
// 与 internal/domain/cronjob 的分工：
//   - 内置任务（scheduler）：随代码发布、无需数据库、不落 cron_jobs 表，由插件自身拥有；
//   - 用户任务（cronjob）：用户在界面上创建，落 cron_jobs 表，支持 HTTP / Shell 动作。
//
// 内置任务必须幂等且可重入安全：默认同一任务上次未结束时跳过本轮（SkipIfBusy）。
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// TaskSpec 是一个内置任务的静态声明。
// 除 Name / Cron / Run 外的字段都有默认值：Timeout 默认 30s，SkipIfBusy 默认 true。
type TaskSpec struct {
	Name string // 展示名，同时作为状态与日志标识（需唯一）
	// Cron 支持 5 字段（分 时 日 月 周）与 6 字段（秒 分 时 日 月 周），与 cronjob 一致。
	Cron       string
	Timeout    time.Duration                   // 单次执行超时；<=0 时取 30s
	RunOnStart bool                            // 启动后立即执行一次（默认等第一次 cron 触发）
	SkipIfBusy bool                            // 上一次未结束时跳过本轮；false 表示排队等待
	Run        func(ctx context.Context) error // 任务体；返回 error 会记录到状态
}

// TaskStatus 是内置任务的运行时状态快照，供只读接口展示。
type TaskStatus struct {
	Name           string     `json:"Name"`
	Cron           string     `json:"Cron"`
	Running        bool       `json:"Running"`
	LastRunAt      *time.Time `json:"LastRunAt"`
	LastDurationMs int64      `json:"LastDurationMs"`
	LastStatus     string     `json:"LastStatus"` // ok / failed / ""
	LastError      string     `json:"LastError"`
	NextRun        *time.Time `json:"NextRun"`
}

// parser 与 cronjob 保持一致：支持可选的秒字段。
var parser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// task 是运行期任务：runMu 保证同一任务串行执行，statusMu 只保护状态快照，
// 两者分离可使状态查询不被长任务阻塞。
type task struct {
	spec     TaskSpec
	entryID  cron.EntryID
	runMu    sync.Mutex
	statusMu sync.Mutex
	status   TaskStatus
}

var (
	mu           sync.RWMutex
	cronInstance *cron.Cron
	tasks        []*task
)

// StartBuiltin 注册并启动全部内置任务。重复调用无副作用（已启动则直接返回）。
// 单条任务声明非法（缺名 / cron 非法）只跳过该任务，不影响其它任务。
func StartBuiltin(specs []TaskSpec) {
	mu.Lock()
	defer mu.Unlock()
	if cronInstance != nil {
		return
	}

	c := cron.New(cron.WithParser(parser))
	for _, spec := range specs {
		if spec.Name == "" || spec.Cron == "" || spec.Run == nil {
			slog.Warn("内置任务声明不完整，已跳过", "name", spec.Name, "cron", spec.Cron)
			continue
		}
		spec := spec
		if spec.Timeout <= 0 {
			spec.Timeout = 30 * time.Second
		}
		schedule, err := parser.Parse(spec.Cron)
		if err != nil {
			slog.Error("内置任务 cron 表达式无效，已跳过", "name", spec.Name, "cron", spec.Cron, "err", err)
			continue
		}
		t := &task{spec: spec, status: TaskStatus{Name: spec.Name, Cron: spec.Cron}}
		t.entryID = c.Schedule(schedule, cron.FuncJob(t.run))
		tasks = append(tasks, t)
	}

	cronInstance = c
	c.Start()
	slog.Info("内置任务调度器启动", "tasks", len(tasks))

	for _, t := range tasks {
		if t.spec.RunOnStart {
			go t.run()
		}
	}
}

// BuiltinStatuses 返回全部内置任务的状态快照（按声明顺序）。
func BuiltinStatuses() []TaskStatus {
	mu.RLock()
	c, list := cronInstance, tasks
	mu.RUnlock()

	entries := map[cron.EntryID]cron.Entry{}
	if c != nil {
		for _, e := range c.Entries() {
			entries[e.ID] = e
		}
	}

	out := make([]TaskStatus, 0, len(list))
	for _, t := range list {
		t.statusMu.Lock()
		st := t.status
		t.statusMu.Unlock()
		if e, ok := entries[t.entryID]; ok && !e.Next.IsZero() {
			next := e.Next
			st.NextRun = &next
		}
		out = append(out, st)
	}
	return out
}

// run 执行一次任务：默认在上一轮未结束时跳过，失败/panic/超时只记录状态，不上抛。
func (t *task) run() {
	if !t.runMu.TryLock() {
		if t.spec.SkipIfBusy {
			slog.Info("内置任务跳过", "name", t.spec.Name, "reason", "上一次还未结束")
			return
		}
		t.runMu.Lock()
	}
	defer t.runMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), t.spec.Timeout)
	defer cancel()

	start := time.Now()
	t.setRunning(true)
	err := safeRun(ctx, t.spec.Run)
	finished := time.Now()

	t.statusMu.Lock()
	t.status.Running = false
	t.status.LastRunAt = &start
	t.status.LastDurationMs = finished.Sub(start).Milliseconds()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("执行超时（%s）", t.spec.Timeout)
		}
		t.status.LastStatus = "failed"
		t.status.LastError = err.Error()
	} else {
		t.status.LastStatus = "ok"
		t.status.LastError = ""
	}
	t.statusMu.Unlock()

	if err != nil {
		slog.Error("内置任务执行失败", "name", t.spec.Name, "duration", finished.Sub(start), "err", err)
		return
	}
	slog.Info("内置任务执行完成", "name", t.spec.Name, "duration", finished.Sub(start))
}

func (t *task) setRunning(v bool) {
	t.statusMu.Lock()
	t.status.Running = v
	t.statusMu.Unlock()
}

// safeRun 把任务 panic 转成 error，避免调度器 goroutine 退出。
func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx)
}
