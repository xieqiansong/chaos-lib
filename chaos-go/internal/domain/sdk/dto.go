package sdk

import "encoding/json"

type SourcePatch struct {
	Sources *json.RawMessage
	Current *string
	Enabled *bool
	Note    *string
}
