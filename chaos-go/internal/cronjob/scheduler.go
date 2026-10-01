package cronjob

import (
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// 本文件是 cron 调度器的运行时实现：加载任务、挂载 / 重排 / 移除条目、计算下次触发时间。
// 只被 service 与启动流程调用。

var (
	// SecondOptional：同时支持 5 字段（分 时 日 月 周）与 6 字段（秒 分 时 日 月 周），
	// 即 cron 表达式可精确到秒级（如 "*/30 * * * * *" 表示每 30 秒）。
	cronParser   = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronInstance *cron.Cron
	cronMu       sync.Mutex
	entryIDs     = map[int]cron.EntryID{}
)

// Start 加载所有启用的定时任务并启动调度器。
func Start() {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance != nil {
		cronInstance.Stop()
	}
	cronInstance = cron.New(cron.WithParser(cronParser))
	jobs, err := FindEnabledJobs()
	if err != nil {
		slog.Error("加载定时任务失败", "err", err)
	}
	for i := range jobs {
		addEntry(&jobs[i])
	}
	cronInstance.Start()
	slog.Info("cronjob 调度器启动", "jobs", len(jobs))
}

// Stop 停止调度器（优雅退出 / 测试用）。
func Stop() {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance != nil {
		cronInstance.Stop()
		cronInstance = nil
	}
	entryIDs = map[int]cron.EntryID{}
}

// SyncJob 在任务变更后重排调度（新增 / 修改 / 启停）。
func SyncJob(job *CronJob) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance == nil {
		return
	}
	if old, ok := entryIDs[job.ID]; ok {
		cronInstance.Remove(old)
		delete(entryIDs, job.ID)
	}
	if job.Enabled && !job.IsDeleted {
		addEntry(job)
	}
}

// RemoveJob 从调度器移除（删除时调用）。
func RemoveJob(id int) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if old, ok := entryIDs[id]; ok {
		cronInstance.Remove(old)
		delete(entryIDs, id)
	}
}

func addEntry(job *CronJob) {
	if cronInstance == nil {
		return
	}
	schedule, err := cronParser.Parse(job.CronExpr)
	if err != nil {
		slog.Error("cron 表达式无效，跳过调度", "jobId", job.ID, "expr", job.CronExpr, "err", err)
		return
	}
	j := *job
	id := cronInstance.Schedule(schedule, cron.FuncJob(func() {
		// 后台触发：执行结果只记运行日志，错误无需上抛。
		_, _ = ExecuteJob(&j)
	}))
	entryIDs[job.ID] = id
}

// NextRuns 计算表达式未来 n 次触发时间（用于预览与列表富化）。
func NextRuns(expr string, n int) ([]time.Time, error) {
	schedule, err := cronParser.Parse(expr)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	runs := make([]time.Time, 0, n)
	t := now
	for i := 0; i < n; i++ {
		t = schedule.Next(t)
		runs = append(runs, t)
	}
	return runs, nil
}
