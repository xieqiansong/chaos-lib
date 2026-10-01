package cronjob

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// preview 参数边界：请求未给或越界时回落到默认条数。
const (
	defaultPreviewCount = 5
	maxPreviewCount     = 20
)

// ── 校验 ────────────────────────────────────────────────────────

// validateJob 校验任务字段：名称 / cron 表达式 / 超时 / 动作配置。
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

// validateAction 按动作类型校验 ActionConfig（JSON 文本）内容是否完整。
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

// ── 用例 ────────────────────────────────────────────────────────

// OnSaved 在 create / update 事务内、提交前执行：先校验（失败即回滚，
// 既不写库也不进调度器），通过后再把最新状态同步进 cron 调度器。
func OnSaved(job *CronJob) error {
	if err := validateJob(job); err != nil {
		return err
	}
	SyncJob(job)
	return nil
}

// RemoveJobOnDelete 删除后把任务移出调度器。
func RemoveJobOnDelete(job *CronJob) error {
	RemoveJob(job.ID)
	return nil
}

// CronJobViews 整批转换为响应视图（crud.Opts.ToResponse）。
func CronJobViews(rows []*CronJob) any {
	out := make([]CronJobView, 0, len(rows))
	for _, job := range rows {
		out = append(out, toView(*job, nextRunOf(job)))
	}
	return out
}

// SetEnabled 启停任务：改库后同步调度器，返回富化视图。
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

// ExecuteNow 立即执行一次任务，返回本次运行记录（避免回查「最新一条」的并发偏差）。
func ExecuteNow(id int) (*CronJobRun, error) {
	job, err := FindJobByID(id)
	if err != nil {
		return nil, err
	}
	return ExecuteJob(&job)
}

// PreviewRuns 校验 cron 表达式并返回未来若干次触发时间（count 越界时回落默认值）。
func PreviewRuns(expr string, count int) ([]time.Time, error) {
	if count <= 0 || count > maxPreviewCount {
		count = defaultPreviewCount
	}
	return NextRuns(expr, count)
}

// nextRunOf 计算任务的下次执行时间；未启用 / 已删除 / 表达式非法时为空。
func nextRunOf(job *CronJob) *time.Time {
	if !job.Enabled || job.IsDeleted {
		return nil
	}
	next, err := NextRuns(job.CronExpr, 1)
	if err != nil || len(next) == 0 {
		return nil
	}
	t := next[0]
	return &t
}
