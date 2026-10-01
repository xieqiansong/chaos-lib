package sdk

import "encoding/json"

// SourcePatch 编辑 SDK 类型的请求体：指针字段用于区分「未提供」与「零值」。
type SourcePatch struct {
	Sources *json.RawMessage
	Current *string
	Enabled *bool
	Note    *string
}
