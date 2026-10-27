package cronjob

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"chaos-go/internal/framework/scheduler"
)

const (
	defaultPreviewCount = 5
	maxPreviewCount     = 20
)

func validateJob(job *CronJob) error {
	if strings.TrimSpace(job.Name) == "" {
		return errors.New("任务名称不能为空")
	}
	if strings.TrimSpace(job.CronExpr) == "" {
		return errors.New("cron 表达式不能为空")
	}
	if _, err := cronParser.Parse(job.CronExpr); err != nil {
		return errors.New("无效的 cron 表达式: " + err.Error())
	}
	if job.TimeoutSec < 0 {
		return errors.New("超时时间不能为负数")
	}
	return validateAction(job)
}

func validateAction(job *CronJob) error {
	switch job.ActionType {
	case ActionTypeHTTP:
		var act HTTPAction
		if err := jsonUnmarshal(job.ActionConfig, &act); err != nil {
			return errors.New("解析 HTTP 动作配置失败: " + err.Error())
		}
		if strings.TrimSpace(act.URL) == "" {
			return errors.New("HTTP 动作的 URL 不能为空")
		}
		return nil
	case ActionTypeShell:
		var act ShellAction
		if err := jsonUnmarshal(job.ActionConfig, &act); err != nil {
			return errors.New("解析命令动作配置失败: " + err.Error())
		}
		if strings.TrimSpace(act.Command) == "" {
			return errors.New("命令动作的命令不能为空")
		}
		return nil
	default:
		return errors.New("未知动作类型: " + string(job.ActionType))
	}
}

func OnSaved(job *CronJob) error {
	if err := validateJob(job); err != nil {
		return err
	}
	SyncJob(job)
	return nil
}

func RemoveJobOnDelete(job *CronJob) error {
	RemoveJob(job.ID)
	return nil
}

func CronJobViews(rows []*CronJob) any {
	out := make([]CronJobView, 0, len(rows))
	for _, job := range rows {
		out = append(out, toView(*job, nextRunOf(job)))
	}
	return out
}

func SetEnabled(id int, enabled bool) (*CronJobView, error) {
	job, err := FindJobByID(id)
	if err != nil {
		return nil, err
	}
	if err := UpdateJobEnabled(job.ID, enabled); err != nil {
		return nil, fmt.Errorf("状态更新失败: %w", err)
	}
	job.Enabled = enabled
	SyncJob(&job)
	view := toView(job, nextRunOf(&job))
	return &view, nil
}

func ExecuteNow(id int) (*CronJobRun, error) {
	job, err := FindJobByID(id)
	if err != nil {
		return nil, err
	}
	return ExecuteJob(&job)
}

func PreviewRuns(expr string, count int) ([]time.Time, error) {
	if count <= 0 || count > maxPreviewCount {
		count = defaultPreviewCount
	}
	return NextRuns(expr, count)
}

func BuiltinTasks() []scheduler.TaskStatus {
	return scheduler.BuiltinStatuses()
}

func nextRunOf(job *CronJob) *time.Time {
	if !job.Enabled || job.Deleted() {
		return nil
	}
	next, err := NextRuns(job.CronExpr, 1)
	if err != nil || len(next) == 0 {
		return nil
	}
	t := next[0]
	return &t
}
