//go:build !windows

package tools

type WtsMessageResult int

const (
	WtsResultOK WtsMessageResult = iota + 1
	WtsResultCancel
	WtsResultYes
	WtsResultNo
	WtsResultTimeout
	WtsResultError
)

type WtsMessageConfig struct {
	Title   string
	Message string
	Style   int
	Timeout int
}

func ShowWtsMessage(config WtsMessageConfig) WtsMessageResult {
	return WtsResultError
}
