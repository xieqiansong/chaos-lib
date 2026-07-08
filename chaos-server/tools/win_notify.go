package tools

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	notifyKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	notifyUser32   = windows.NewLazySystemDLL("user32.dll")
	notifyGdi32    = windows.NewLazySystemDLL("gdi32.dll")
	notifyWinmm    = windows.NewLazySystemDLL("winmm.dll")

	notifyProcGetModuleHandle = notifyKernel32.NewProc("GetModuleHandleW")
	notifyProcCreateWindowEx  = notifyUser32.NewProc("CreateWindowExW")
	notifyProcDefWindowProc   = notifyUser32.NewProc("DefWindowProcW")
	notifyProcRegisterClassEx = notifyUser32.NewProc("RegisterClassExW")
	notifyProcLoadCursor      = notifyUser32.NewProc("LoadCursorW")
	notifyProcShowWindow      = notifyUser32.NewProc("ShowWindow")
	notifyProcUpdateWindow    = notifyUser32.NewProc("UpdateWindow")
	notifyProcGetMessage      = notifyUser32.NewProc("GetMessageW")
	notifyProcTranslateMsg    = notifyUser32.NewProc("TranslateMessage")
	notifyProcDispatchMsg     = notifyUser32.NewProc("DispatchMessageW")
	notifyProcDestroyWindow   = notifyUser32.NewProc("DestroyWindow")
	notifyProcPostQuitMsg     = notifyUser32.NewProc("PostQuitMessage")
	notifyProcBeginPaint      = notifyUser32.NewProc("BeginPaint")
	notifyProcEndPaint        = notifyUser32.NewProc("EndPaint")
	notifyProcGetDC           = notifyUser32.NewProc("GetDC")
	notifyProcReleaseDC       = notifyUser32.NewProc("ReleaseDC")
	notifyProcFillRect        = notifyUser32.NewProc("FillRect")
	notifyProcDrawText        = notifyUser32.NewProc("DrawTextW")
	notifyProcGetSysMetrics   = notifyUser32.NewProc("GetSystemMetrics")
	notifyProcSetWindowRgn    = notifyUser32.NewProc("SetWindowRgn")
	notifyProcSendMessageW    = notifyUser32.NewProc("SendMessageW")
	notifyProcPlaySound       = notifyWinmm.NewProc("PlaySoundW")

	notifyProcCreateSolidBrush     = notifyGdi32.NewProc("CreateSolidBrush")
	notifyProcCreatePen            = notifyGdi32.NewProc("CreatePen")
	notifyProcDeleteObject         = notifyGdi32.NewProc("DeleteObject")
	notifyProcSelectObject         = notifyGdi32.NewProc("SelectObject")
	notifyProcGetStockObject       = notifyGdi32.NewProc("GetStockObject")
	notifyProcCreateRoundRectRgn   = notifyGdi32.NewProc("CreateRoundRectRgn")
	notifyProcRoundRect            = notifyGdi32.NewProc("RoundRect")
	notifyProcSetBkMode            = notifyGdi32.NewProc("SetBkMode")
	notifyProcSetTextColor         = notifyGdi32.NewProc("SetTextColor")
	notifyProcGetTextExtentPoint32 = notifyGdi32.NewProc("GetTextExtentPoint32W")
)

const (
	notifyWS_POPUP         = 0x80000000
	notifyWS_VISIBLE       = 0x10000000
	notifyWS_CLIPCHILDREN  = 0x02000000
	notifyWS_EX_TOOLWINDOW = 0x00000080
	notifyWS_EX_NOACTIVATE = 0x08000000
	notifyWS_EX_TOPMOST    = 0x00000008
	notifyWS_CHILD         = 0x40000000
	notifyBS_OWNERDRAW     = 0x0000000B

	notifyWM_CREATE   = 0x0001
	notifyWM_DESTROY  = 0x0002
	notifyWM_COMMAND  = 0x0111
	notifyWM_PAINT    = 0x000F
	notifyWM_CLOSE    = 0x0010
	notifyWM_SETFONT  = 0x0030
	notifyWM_DRAWITEM = 0x002B

	notifySW_SHOWNOACTIVATE = 4

	notifyID_OK     = 1001
	notifyID_CANCEL = 1002
	notifyID_CLOSE  = 1003

	notifyIDC_ARROW = 32512

	notifyDT_CENTER     = 0x0001
	notifyDT_VCENTER    = 0x0004
	notifyDT_SINGLELINE = 0x0020
	notifyDT_WORDBREAK  = 0x00000010
	notifyDT_NOPREFIX   = 0x00000800

	notifyODS_SELECTED = 0x0001
	notifyODS_HOTLIGHT = 0x0040
	notifyODS_DISABLED = 0x0004

	notifyDEFAULT_GUI_FONT = 17

	notifyTRANSPARENT = 1
	notifyPS_SOLID    = 0

	// PlaySound 标志位：使用系统别名 + 异步播放
	notifySND_ALIAS     = 0x00010000
	notifySND_ASYNC     = 0x00000001
	notifySND_NODEFAULT = 0x00000002
)

type notifyWNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type notifyMSG struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type notifyPAINTSTRUCT struct {
	Hdc         windows.Handle
	FErase      int32
	RcPaint     struct{ L, T, R, B int32 }
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type notifyRECT struct {
	L, T, R, B int32
}

type notifyDRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemState  uint32
	ItemAction uint32
	HwndItem   windows.Handle
	Hdc        windows.Handle
	RcItem     notifyRECT
	ItemData   uintptr
}

type notifySIZE struct {
	Cx, Cy int32
}

// NotifyConfig 通知窗口配置
type NotifyConfig struct {
	Title       string
	Text        string
	Width       int32
	Height      int32
	OKLabel     string
	CancelLabel string
}

// NotifyResult 用户操作结果
type NotifyResult int

const (
	NotifyOK NotifyResult = iota + 1
	NotifyCancel
	NotifyClose
)

var (
	notifyCurrentConfig NotifyConfig
	notifyResultCh      chan NotifyResult
	notifyHMainWnd      windows.Handle
	notifyDefaultFont   windows.Handle
	notifyMu            sync.Mutex // 防止多个 goroutine 并发调用 ShowNotify 时共享变量冲突
)

// rgb 构造 COLORREF（uintptr）：0x00BBGGRR
func rgb(r, g, b uint32) uintptr {
	return uintptr(r | (g << 8) | (b << 16))
}

// ShowNotify 右下角弹出一个带"确定/取消/关闭"的通知窗口，并阻塞等待用户响应。
// 返回用户点击的按钮；若窗口被系统关闭，返回 NotifyClose。
// 只能在一个 OS 线程上调用；内部会自行 LockOSThread。
// 注意：内部使用互斥锁串行化并发调用，避免多个 goroutine 同时弹窗时共享变量冲突。
func ShowNotify(config NotifyConfig) NotifyResult {
	notifyMu.Lock()
	defer notifyMu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if config.Title == "" {
		config.Title = "通知"
	}
	if config.Text == "" {
		config.Text = "您有一条新通知。"
	}
	if config.Width == 0 {
		config.Width = 320
	}
	if config.Height == 0 {
		config.Height = 116
	}
	if config.OKLabel == "" {
		config.OKLabel = "确定"
	}
	if config.CancelLabel == "" {
		config.CancelLabel = "取消"
	}

	notifyCurrentConfig = config
	resultCh := make(chan NotifyResult, 1)
	notifyResultCh = resultCh

	// 获取 Windows 默认 GUI 字体
	hFontRaw, _, _ := notifyProcGetStockObject.Call(uintptr(notifyDEFAULT_GUI_FONT))
	notifyDefaultFont = windows.Handle(hFontRaw)

	className := windows.StringToUTF16Ptr("MyNotifyClass")
	windowTitle := windows.StringToUTF16Ptr(config.Title)

	hInstRaw, _, _ := notifyProcGetModuleHandle.Call(0)
	if hInstRaw == 0 {
		fmt.Println("GetModuleHandleW failed")
		return NotifyClose
	}
	hInst := windows.Handle(hInstRaw)

	hCursor, _, _ := notifyProcLoadCursor.Call(0, notifyIDC_ARROW)
	// 背景用 NULL 画刷（= 0），完全由我们自绘，避免系统默认底色
	wc := notifyWNDCLASSEX{
		Size:       uint32(unsafe.Sizeof(notifyWNDCLASSEX{})),
		Style:      0,
		WndProc:    windows.NewCallback(notifyWindowProc),
		ClsExtra:   0,
		WndExtra:   0,
		Instance:   hInst,
		Icon:       0,
		Cursor:     windows.Handle(hCursor),
		Background: windows.Handle(0), // 不使用系统颜色，我们自己绘
		MenuName:   nil,
		ClassName:  className,
		IconSm:     0,
	}
	ret, _, err := notifyProcRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		// 重复注册可忽略；其它错误打印一下，便于排错
		fmt.Printf("RegisterClassEx 返回 0: %v\n", err)
	}

	screenW := int(notifyGetSysMetrics(0))
	screenH := int(notifyGetSysMetrics(1))
	x := int32(screenW) - config.Width - 20
	y := int32(screenH) - config.Height - 70

	hWnd, _, err2 := notifyProcCreateWindowEx.Call(
		uintptr(notifyWS_EX_TOOLWINDOW|notifyWS_EX_NOACTIVATE|notifyWS_EX_TOPMOST),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		uintptr(notifyWS_POPUP|notifyWS_VISIBLE|notifyWS_CLIPCHILDREN),
		uintptr(x), uintptr(y),
		uintptr(config.Width), uintptr(config.Height),
		0, 0, uintptr(hInst), 0,
	)
	if hWnd == 0 {
		fmt.Printf("CreateWindowEx failed: %v\n", err2)
		return NotifyClose
	}
	notifyHMainWnd = windows.Handle(hWnd)

	// 设置圆角窗口区域，让窗口本身就变成圆角形状（避免"融入白色"）
	const cornerRadius int32 = 8
	hRgn, _, _ := notifyProcCreateRoundRectRgn.Call(0, 0, uintptr(config.Width+1), uintptr(config.Height+1), uintptr(cornerRadius), uintptr(cornerRadius))
	if hRgn != 0 {
		// SetWindowRgn 会接管 region 句柄，无需再 DeleteObject
		notifyProcSetWindowRgn.Call(hWnd, hRgn, 1)
	}

	notifyProcShowWindow.Call(hWnd, notifySW_SHOWNOACTIVATE)
	notifyProcUpdateWindow.Call(hWnd)

	// 播放系统默认通知声音（SystemNotification），异步播放，不阻塞窗口流程
	notifyPlaySystemSound("SystemNotification")

	var msg notifyMSG
	for {
		r, _, _ := notifyProcGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 {
			break
		}
		notifyProcTranslateMsg.Call(uintptr(unsafe.Pointer(&msg)))
		notifyProcDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))

		select {
		case res := <-resultCh:
			notifyProcDestroyWindow.Call(hWnd)
			for {
				r2, _, _ := notifyProcGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
				if r2 == 0 {
					return res
				}
				notifyProcTranslateMsg.Call(uintptr(unsafe.Pointer(&msg)))
				notifyProcDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))
			}
		default:
		}
	}

	select {
	case res := <-resultCh:
		return res
	default:
		return NotifyClose
	}
}

func notifyWindowProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case notifyWM_CREATE:
		hInstRaw, _, _ := notifyProcGetModuleHandle.Call(0)
		hInst := windows.Handle(hInstRaw)
		cfg := notifyCurrentConfig

		// 按钮布局：两个主按钮居下，"x" 在右上角
		okX := int32(14)
		okY := cfg.Height - 34
		okW := cfg.Width/2 - 21
		okH := int32(28)

		cancelX := cfg.Width/2 + 7
		cancelY := okY
		cancelW := okW
		cancelH := okH

		closeW := int32(24)
		closeH := int32(24)
		closeX := cfg.Width - closeW - 4
		closeY := int32(4)

		notifyCreateButton(hwnd, hInst, cfg.OKLabel, okX, okY, okW, okH, notifyID_OK)
		notifyCreateButton(hwnd, hInst, cfg.CancelLabel, cancelX, cancelY, cancelW, cancelH, notifyID_CANCEL)
		notifyCreateButton(hwnd, hInst, "×", closeX, closeY, closeW, closeH, notifyID_CLOSE)
		return 0

	case notifyWM_COMMAND:
		id := int32(wParam & 0xFFFF)
		switch id {
		case notifyID_OK:
			notifyPostResult(NotifyOK)
		case notifyID_CANCEL:
			notifyPostResult(NotifyCancel)
		case notifyID_CLOSE:
			notifyPostResult(NotifyClose)
		}
		return 0

	case notifyWM_CLOSE:
		notifyPostResult(NotifyClose)
		return 0

	case notifyWM_DRAWITEM:
		// 自绘圆角按钮
		dis := (*notifyDRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		notifyDrawButton(dis)
		return 0

	case notifyWM_PAINT:
		var ps notifyPAINTSTRUCT
		hdcRaw, _, _ := notifyProcBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdcRaw != 0 {
			cfg := notifyCurrentConfig
			notifyDrawNotification(hdcRaw, cfg)
			notifyProcEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0

	case notifyWM_DESTROY:
		notifyProcPostQuitMsg.Call(0)
		return 0
	}
	ret, _, _ := notifyProcDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// notifyDrawNotification 绘制通知主体：白底 + 浅灰边框 + 标题 + 正文
func notifyDrawNotification(hdc uintptr, cfg NotifyConfig) {
	// 选入默认字体；保存旧字体以便还原
	oldFont, _, _ := notifyProcSelectObject.Call(hdc, uintptr(notifyDefaultFont))

	// 透明背景模式（让 DrawText 不覆盖我们画的底色）
	notifyProcSetBkMode.Call(hdc, notifyTRANSPARENT)

	// 白色背景填充整个客户区
	brushWhite, _, _ := notifyProcCreateSolidBrush.Call(rgb(255, 255, 255))
	if brushWhite != 0 {
		fullRect := notifyRECT{0, 0, cfg.Width, cfg.Height}
		notifyProcFillRect.Call(hdc, uintptr(unsafe.Pointer(&fullRect)), brushWhite)
		notifyProcDeleteObject.Call(brushWhite)
	}

	// 画一层浅灰细边框，避免在纯白色背景下"融为一体"
	penBorder, _, _ := notifyProcCreatePen.Call(notifyPS_SOLID, 1, rgb(225, 225, 225))
	if penBorder != 0 {
		oldPen, _, _ := notifyProcSelectObject.Call(hdc, penBorder)
		brushNull, _, _ := notifyProcGetStockObject.Call(5) // HOLLOW_BRUSH = 5
		oldBrush, _, _ := notifyProcSelectObject.Call(hdc, brushNull)
		const cr int32 = 8
		notifyProcRoundRect.Call(hdc, 0, 0, uintptr(cfg.Width), uintptr(cfg.Height), uintptr(cr), uintptr(cr))
		notifyProcSelectObject.Call(hdc, oldBrush)
		notifyProcSelectObject.Call(hdc, oldPen)
		notifyProcDeleteObject.Call(penBorder)
	}

	// 标题：深色
	notifyProcSetTextColor.Call(hdc, rgb(20, 20, 20))
	titlePtr := windows.StringToUTF16Ptr(cfg.Title)
	titleRect := notifyRECT{14, 10, cfg.Width - 42, 32}
	notifyProcDrawText.Call(
		hdc,
		uintptr(unsafe.Pointer(titlePtr)),
		^uintptr(0),
		uintptr(unsafe.Pointer(&titleRect)),
		uintptr(notifyDT_SINGLELINE|notifyDT_VCENTER|notifyDT_NOPREFIX),
	)

	// 正文：中灰色、自动换行（约 2 行空间，紧贴按钮上方）
	notifyProcSetTextColor.Call(hdc, rgb(90, 90, 90))
	textPtr := windows.StringToUTF16Ptr(cfg.Text)
	textRect := notifyRECT{14, 36, cfg.Width - 14, cfg.Height - 38}
	notifyProcDrawText.Call(
		hdc,
		uintptr(unsafe.Pointer(textPtr)),
		^uintptr(0),
		uintptr(unsafe.Pointer(&textRect)),
		uintptr(notifyDT_WORDBREAK|notifyDT_NOPREFIX),
	)

	// 还原旧字体
	notifyProcSelectObject.Call(hdc, oldFont)
}

// notifyDrawButton 自绘圆角按钮：根据 hover/pressed 状态改变底色与边框
func notifyDrawButton(dis *notifyDRAWITEMSTRUCT) {
	hdc := uintptr(dis.Hdc)
	r := dis.RcItem
	state := dis.ItemState

	// 按钮矩形
	x1 := r.L
	y1 := r.T
	x2 := r.R
	y2 := r.B
	h := y2 - y1

	// 圆角半径，按按钮高度自适应（越小越锐利）
	var cr int32 = 8
	if h < 28 {
		cr = 6
	}

	// 颜色方案（现代 Windows 风）
	var fillColor, borderColor, textColor uintptr
	if (state & notifyODS_SELECTED) != 0 {
		fillColor = rgb(220, 230, 248)
		borderColor = rgb(140, 170, 220)
		textColor = rgb(15, 15, 15)
	} else if (state & notifyODS_HOTLIGHT) != 0 {
		fillColor = rgb(238, 245, 253)
		borderColor = rgb(180, 205, 240)
		textColor = rgb(15, 15, 15)
	} else {
		fillColor = rgb(249, 249, 249)
		borderColor = rgb(210, 210, 210)
		textColor = rgb(30, 30, 30)
	}

	// 选入默认字体
	oldFont, _, _ := notifyProcSelectObject.Call(hdc, uintptr(notifyDefaultFont))

	// 透明文字背景
	notifyProcSetBkMode.Call(hdc, notifyTRANSPARENT)

	// 画填充圆角矩形（先刷后描边，避免边缘被覆盖）
	brush, _, _ := notifyProcCreateSolidBrush.Call(fillColor)
	if brush != 0 {
		oldBrush, _, _ := notifyProcSelectObject.Call(hdc, brush)
		pen, _, _ := notifyProcCreatePen.Call(notifyPS_SOLID, 1, borderColor)
		if pen != 0 {
			oldPen, _, _ := notifyProcSelectObject.Call(hdc, pen)
			notifyProcRoundRect.Call(hdc,
				uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2),
				uintptr(cr*2), uintptr(cr*2))
			notifyProcSelectObject.Call(hdc, oldPen)
			notifyProcDeleteObject.Call(pen)
		}
		notifyProcSelectObject.Call(hdc, oldBrush)
		notifyProcDeleteObject.Call(brush)
	}

	// 画按钮文字：居中 + 垂直居中
	notifyProcSetTextColor.Call(hdc, textColor)

	lenRaw, _, _ := notifyProcSendMessageW.Call(uintptr(dis.HwndItem), 0x000E, 0, 0) // WM_GETTEXTLENGTH
	textLen := int32(lenRaw)
	if textLen > 0 {
		buf := make([]uint16, textLen+1)
		notifyProcSendMessageW.Call(uintptr(dis.HwndItem), 0x000D, uintptr(textLen+1), uintptr(unsafe.Pointer(&buf[0]))) // WM_GETTEXT
		textRect := notifyRECT{x1, y1, x2, y2}
		notifyProcDrawText.Call(
			hdc,
			uintptr(unsafe.Pointer(&buf[0])),
			^uintptr(0),
			uintptr(unsafe.Pointer(&textRect)),
			uintptr(notifyDT_CENTER|notifyDT_VCENTER|notifyDT_SINGLELINE|notifyDT_NOPREFIX),
		)
	}

	notifyProcSelectObject.Call(hdc, oldFont)
}

func notifyPostResult(res NotifyResult) {
	defer func() { recover() }()
	if notifyResultCh != nil {
		select {
		case notifyResultCh <- res:
		default:
		}
	}
}

func notifyCreateButton(parent, instance windows.Handle, text string, x, y, w, h int32, id int32) windows.Handle {
	btnText := windows.StringToUTF16Ptr(text)
	// 自绘按钮：BS_OWNERDRAW + WS_CHILD + WS_VISIBLE
	hWnd, _, _ := notifyProcCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(btnText)),
		uintptr(notifyWS_CHILD|notifyWS_VISIBLE|notifyBS_OWNERDRAW),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent),
		uintptr(id),
		uintptr(instance),
		0,
	)
	// 给按钮设置默认字体，确保文本测量与绘制使用系统字体
	if hWnd != 0 {
		notifyProcSendMessageW.Call(hWnd, notifyWM_SETFONT, uintptr(notifyDefaultFont), 0)
	}
	return windows.Handle(hWnd)
}

func notifyGetSysMetrics(index int32) int {
	ret, _, _ := notifyProcGetSysMetrics.Call(uintptr(index))
	return int(ret)
}

// notifyPlaySystemSound 通过 winmm!PlaySoundW 播放 Windows 系统声音。
// soundName 是系统声音别名，常用值：
//   - "SystemNotification"（默认通知音，Windows 默认通知音
//   - "SystemAsterisk"（提示音）
//   - "SystemExclamation"（警告提示）
//   - "SystemHand"（错误）
//   - "SystemQuestion"（疑问）
//   - "SystemStart"（启动音）
//
// 若系统未配置对应声音会静默返回，不影响主流程。
func notifyPlaySystemSound(soundName string) {
	name := windows.StringToUTF16Ptr(soundName)
	// SND_ALIAS | SND_ASYNC | SND_NODEFAULT
	notifyProcPlaySound.Call(
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(notifySND_ALIAS|notifySND_ASYNC|notifySND_NODEFAULT),
	)
}
