package cronjob

import (
	"errors"
	"time"

	"chaos-go/internal/config"
	"chaos-go/internal/pagination"
	"gorm.io/gorm"
)

// FindJobByID 读取未删除的指定任务；不存在返回 ErrJobNotFound。
func FindJobByID(id int) (CronJob, error) {
	var job CronJob
	if err := config.GetDB().Where("is_deleted = ?", false).First(&job, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CronJob{}, ErrJobNotFound
		}
		return CronJob{}, err
	}
	return job, nil
}

// FindEnabledJobs 读取所有启用且未删除的任务（调度器启动时加载）。
func FindEnabledJobs() ([]CronJob, error) {
	var jobs []CronJob
	if err := config.GetDB().Where("enabled = ? AND is_deleted = ?", true, false).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// UpdateJobEnabled 更新任务启用状态。
func UpdateJobEnabled(id int, enabled bool) error {
	return config.GetDB().Model(&CronJob{}).Where("id = ?", id).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

// CreateRun 写入一条运行日志。
func CreateRun(run *CronJobRun) error {
	return config.GetDB().Create(run).Error
}

// UpdateJobRunResult 回写任务的末次运行时间与状态。
func UpdateJobRunResult(jobID int, finished time.Time, status string) error {
	return config.GetDB().Model(&CronJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"last_run_at": &finished,
		"last_status": status,
		"updated_at":  finished,
	}).Error
}

// FindLatestRun 读取某任务最近一次运行记录。
func FindLatestRun(jobID int) (CronJobRun, error) {
	var latest CronJobRun
	if err := config.GetDB().Where("job_id = ?", jobID).Order("started_at DESC").First(&latest).Error; err != nil {
		return CronJobRun{}, err
	}
	return latest, nil
}

// FindRuns 分页查询某任务的运行历史。
func FindRuns(jobID int, q pagination.Query) ([]CronJobRun, int64, error) {
	var list []CronJobRun
	base := config.GetDB().Model(&CronJobRun{}).Where("job_id = ?", jobID).Order("started_at DESC")
	total, err := pagination.Paginate(base, &list, q)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// CountJobs 统计未删除的任务数量（用于判断是否需写入默认任务）。
func CountJobs() (int64, error) {
	var count int64
	if err := config.GetDB().Model(&CronJob{}).Where("is_deleted = ?", false).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateJobs 批量写入默认任务。
func CreateJobs(jobs []CronJob) error {
	return config.GetDB().Create(&jobs).Error
}
