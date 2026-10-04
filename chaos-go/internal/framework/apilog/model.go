package apilog

import "chaos-go/internal/framework/crud"

// ApiLog 单条接口访问日志（标准数据范式：嵌入 BaseModel 获得 ID/创建/更新/软删）。
type ApiLog struct {
	crud.BaseModel
	Path       string `json:"Path"`
	Method     string `json:"Method"`
	StatusCode int    `json:"StatusCode"`
	LatencyMs  int64  `json:"LatencyMs"` // 相应用时（毫秒）
	ClientIP   string `json:"ClientIP"`
	UserAgent  string `json:"UserAgent"`
	BodySize   int    `json:"BodySize"`
	ErrorMsg   string `json:"ErrorMsg"`
}

// TableName 显式指定表名（与前端资源名 apiLog 对应）。
func (ApiLog) TableName() string { return "api_logs" }
