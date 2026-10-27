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

type TaskSpec struct {
	Name string

	Cron       string
	Timeout    time.Duration
	RunOnStart bool
	SkipIfBusy bool
	Run        func(ctx context.Context) error
}

type TaskStatus struct {
	Name           string     `json:"Name"`
	Cron           string     `json:"Cron"`
	Running        bool       `json:"Running"`
	LastRunAt      *time.Time `json:"LastRunAt"`
	LastDurationMs int64      `json:"LastDurationMs"`
	LastStatus     string     `json:"LastStatus"`
	LastError      string     `json:"LastError"`
	NextRun        *time.Time `json:"NextRun"`
}

var parser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

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

func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx)
}
