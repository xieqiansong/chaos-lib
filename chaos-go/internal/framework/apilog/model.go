package apilog

import "chaos-go/internal/framework/crud"

type ApiLog struct {
	crud.BaseModel
	Path       string `json:"Path"`
	Method     string `json:"Method"`
	StatusCode int    `json:"StatusCode"`
	LatencyMs  int64  `json:"LatencyMs"`
	ClientIP   string `json:"ClientIP"`
	UserAgent  string `json:"UserAgent"`
	BodySize   int    `json:"BodySize"`
	ErrorMsg   string `json:"ErrorMsg"`
}

func (ApiLog) TableName() string { return "api_logs" }
