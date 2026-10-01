package apilog

import "chaos-go/internal/config"

// persistBatch 将一批接口日志批量写入数据库。
func persistBatch(batch []*ApiLog) error {
	if db := config.GetDB(); db != nil {
		return db.CreateInBatches(batch, len(batch)).Error
	}
	return nil
}
