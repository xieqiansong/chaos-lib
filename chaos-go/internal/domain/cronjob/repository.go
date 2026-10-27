package cronjob

import (
	"errors"
	"time"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/pagination"
	"gorm.io/gorm"
)

func FindJobByID(id int) (CronJob, error) {
	var job CronJob
	if err := config.GetDB().First(&job, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CronJob{}, ErrJobNotFound
		}
		return CronJob{}, err
	}
	return job, nil
}

func FindEnabledJobs() ([]CronJob, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var jobs []CronJob
	if err := db.Where("enabled = ?", true).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func UpdateJobEnabled(id int, enabled bool) error {
	return config.GetDB().Model(&CronJob{}).Where("id = ?", id).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

func CreateRun(run *CronJobRun) error {
	return config.GetDB().Create(run).Error
}

func UpdateJobRunResult(jobID int, finished time.Time, status string) error {
	return config.GetDB().Model(&CronJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"last_run_at": &finished,
		"last_status": status,
		"updated_at":  finished,
	}).Error
}

func FindLatestRun(jobID int) (CronJobRun, error) {
	var latest CronJobRun
	if err := config.GetDB().Where("job_id = ?", jobID).Order("started_at DESC").First(&latest).Error; err != nil {
		return CronJobRun{}, err
	}
	return latest, nil
}

func FindRuns(jobID int, q pagination.Query) ([]CronJobRun, int64, error) {
	var list []CronJobRun
	base := config.GetDB().Model(&CronJobRun{}).Where("job_id = ?", jobID).Order("started_at DESC")
	total, err := pagination.Paginate(base, &list, q)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func CountJobs() (int64, error) {
	var count int64
	if err := config.GetDB().Model(&CronJob{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func CreateJobs(jobs []CronJob) error {
	return config.GetDB().Create(&jobs).Error
}
