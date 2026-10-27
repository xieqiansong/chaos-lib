package cronjob

import (
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	cronParser   = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronInstance *cron.Cron
	cronMu       sync.Mutex
	entryIDs     = map[int]cron.EntryID{}
)

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

func Stop() {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance != nil {
		cronInstance.Stop()
		cronInstance = nil
	}
	entryIDs = map[int]cron.EntryID{}
}

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
	if job.Enabled && !job.Deleted() {
		addEntry(job)
	}
}

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

		_, _ = ExecuteJob(&j)
	}))
	entryIDs[job.ID] = id
}

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
