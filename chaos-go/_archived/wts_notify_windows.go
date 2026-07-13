package tools

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procWTSSendMessageW      = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSSendMessageW")
	procWTSQuerySessionInfoW = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSQuerySessionInformationW")
	procProcessIdToSessionId = windows.NewLazySystemDLL("kernel32.dll").NewProc("ProcessIdToSessionId")

	isServiceCached  bool
	serviceCheckDone bool
)

const (
	WTS_CURRENT_SERVER_HANDLE = 0
	WTS_CURRENT_SESSION       = 0xFFFFFFFF

	// MessageBox 样式
	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNO           = 0x00000004
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONERROR       = 0x00000010
	MB_ICONQUESTION    = 0x00000020
	MB_TOPMOST         = 0x00040000
	MB_SYSTEMMODAL     = 0x00001000
	MB_SETFOREGROUND   = 0x00010000
	MB_DEFBUTTON2      = 0x00000100

	// WTSSendMessage 返回值
	IDOK      = 1
	IDCANCEL  = 2
	IDYES     = 6
	IDNO      = 7
	IDTIMEOUT = 32000
)

// WtsMessageResult 用户响应
type WtsMessageResult int

const (
	WtsResultOK WtsMessageResult = iota + 1
	WtsResultCancel
	WtsResultYes
	WtsResultNo
	WtsResultTimeout
	WtsResultError
)

// WtsMessageConfig 消息框配置
type WtsMessageConfig struct {
	Title   string
	Message string
	Style   int // MB_OKCANCEL | MB_ICONINFORMATION | MB_TOPMOST 等
	Timeout int // 0 = 无限等待
}

// ShowWtsMessage 通过 WTSSendMessage 在用户桌面弹出系统对话框。
// 即使当前进程运行在 Session 0（如 nssm 服务），也能正确显示在用户会话中。
// 返回值：用户点击的按钮类型。
func ShowWtsMessage(config WtsMessageConfig) WtsMessageResult {
	if config.Title == "" {
		config.Title = "通知"
	}
	if config.Style == 0 {
		config.Style = MB_OKCANCEL | MB_ICONINFORMATION | MB_TOPMOST | MB_SYSTEMMODAL | MB_SETFOREGROUND
	}

	titlePtr, _ := windows.UTF16PtrFromString(config.Title)
	msgPtr, _ := windows.UTF16PtrFromString(config.Message)

	titleLen := uint32(len(syscall.StringToUTF16(config.Title)))
	msgLen := uint32(len(syscall.StringToUTF16(config.Message)))

	var response uint32
	r1, _, err := procWTSSendMessageW.Call(
		WTS_CURRENT_SERVER_HANDLE,
		WTS_CURRENT_SESSION,
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(titleLen),
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(msgLen),
		uintptr(config.Style),
		uintptr(config.Timeout),
		uintptr(unsafe.Pointer(&response)),
		1, // bWait = TRUE
	)
	if r1 == 0 {
		fmt.Printf("WTSSendMessage 失败: %v\n", err)
		return WtsResultError
	}

	switch response {
	case IDOK:
		return WtsResultOK
	case IDCANCEL:
		return WtsResultCancel
	case IDYES:
		return WtsResultYes
	case IDNO:
		return WtsResultNo
	case IDTIMEOUT:
		return WtsResultTimeout
	default:
		return WtsResultError
	}
}

// IsRunningAsService 检测当前进程是否运行在 Session 0（服务模式）。
// Session 0 中的进程无法直接显示 GUI 窗口给用户。
func IsRunningAsService() bool {
	if serviceCheckDone {
		return isServiceCached
	}
	serviceCheckDone = true

	var sessionId uint32
	r1, _, _ := procProcessIdToSessionId.Call(
		uintptr(windows.GetCurrentProcessId()),
		uintptr(unsafe.Pointer(&sessionId)),
	)
	if r1 == 0 {
		isServiceCached = false
		return false
	}
	isServiceCached = (sessionId == 0)
	return isServiceCached
}

// ShowNotifyAuto 自动选择通知方式：服务模式用 WTSSendMessage，否则用自定义窗口。
// serviceOK 和 serviceCancel 为服务模式下确定/取消按钮的文字。
func ShowNotifyAuto(title, text, okLabel, cancelLabel string) NotifyResult {
	if IsRunningAsService() {
		style := MB_OKCANCEL | MB_ICONINFORMATION | MB_TOPMOST | MB_SYSTEMMODAL | MB_SETFOREGROUND
		result := ShowWtsMessage(WtsMessageConfig{
			Title:   title,
			Message: text,
			Style:   style,
		})
		switch result {
		case WtsResultOK:
			return NotifyOK
		case WtsResultCancel:
			return NotifyCancel
		default:
			return NotifyClose
		}
	}

	return ShowNotify(NotifyConfig{
		Title:       title,
		Text:        text,
		OKLabel:     okLabel,
		CancelLabel: cancelLabel,
	})
}
