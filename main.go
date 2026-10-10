package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"os"
	"syscall"
	"unsafe"

	webview "github.com/jchv/go-webview2"
)

//go:embed index.html
var htmlContent string

const (
	GWL_STYLE        = 0xFFFFFFF0 // -16
	WS_CAPTION       = 0x00C00000
	WS_THICKFRAME    = 0x00040000
	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOZORDER     = 0x0004
	WM_NCLBUTTONDOWN = 0x00A1
	HTCAPTION        = 2

	SW_HIDE     = 0
	SW_SHOW     = 5
	SW_MINIMIZE = 6
	SW_MAXIMIZE = 3
	SW_RESTORE  = 9

	// Windows 11 DWM 原生圆角属性
	DWMWA_WINDOW_CORNER_PREFERENCE = 33
	DWMWCP_ROUND                   = 2 // 圆角窗口
)

type MARGINS struct {
	CxLeftWidth    int32
	CxRightWidth   int32
	CyTopHeight    int32
	CyBottomHeight int32
}

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	dwmapi             = syscall.NewLazyDLL("dwmapi.dll")
	procShowWindow     = user32.NewProc("ShowWindow")
	procGetWindowLong  = user32.NewProc("GetWindowLongW")
	procSetWindowLong  = user32.NewProc("SetWindowLongW")
	procSetWindowPos   = user32.NewProc("SetWindowPos")
	procReleaseCapture = user32.NewProc("ReleaseCapture")
	procSendMessage    = user32.NewProc("SendMessageW")

	procDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procDwmSetWindowAttribute        = dwmapi.NewProc("DwmSetWindowAttribute")

	comctl32              = syscall.NewLazyDLL("comctl32.dll")
	procSetWindowSubclass = comctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc   = comctl32.NewProc("DefSubclassProc")
)

func main() {
	// 关键优化 1：直接在 WindowOptions 中锁定 920x650 并居中屏幕，彻底消除“先小窗口再变大”的缩放闪烁现象
	w := webview.NewWithOptions(webview.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview.WindowOptions{
			Title:  "PortKiller Pro",
			Width:  920,
			Height: 650,
			Center: true,
		},
	})
	if w == nil {
		log.Fatal("Failed to initialize WebView2 window. Ensure Microsoft Edge WebView2 runtime is available.")
	}
	defer w.Destroy()

	hwnd := uintptr(w.Window())

	// 先隐蔽窗体进行样式配置，避免界面初次绘制时的视觉抖动
	procShowWindow.Call(hwnd, uintptr(SW_HIDE))

	// 关键优化 2：去除 Windows 原生白底标题栏 (WS_CAPTION)，保留 DWM 缩放与阴影
	style, _, _ := procGetWindowLong.Call(hwnd, uintptr(GWL_STYLE))
	newStyle := (style &^ WS_CAPTION) | WS_THICKFRAME
	procSetWindowLong.Call(hwnd, uintptr(GWL_STYLE), newStyle)

	// 关键优化 3：子类化拦截 WM_NCCALCSIZE，彻底消除系统自带的 8px 白色非客户区边距
	subclassCallback := syscall.NewCallback(func(hWnd uintptr, uMsg uint32, wParam, lParam, uIdSubclass, dwRefData uintptr) uintptr {
		if uMsg == 0x0083 { // WM_NCCALCSIZE
			if wParam != 0 {
				// 返回 0 让客户端区域 100% 填满窗口矩形，杜绝任何外部白条与空隙
				return 0
			}
		}
		r, _, _ := procDefSubclassProc.Call(hWnd, uintptr(uMsg), wParam, lParam)
		return r
	})
	procSetWindowSubclass.Call(hwnd, subclassCallback, 1, 0)

	// 关键优化 4：激活 Windows 11 原生硬件 DWM 圆角渲染
	var cornerPreference uint32 = DWMWCP_ROUND
	procDwmSetWindowAttribute.Call(
		hwnd,
		uintptr(DWMWA_WINDOW_CORNER_PREFERENCE),
		uintptr(unsafe.Pointer(&cornerPreference)),
		uintptr(unsafe.Sizeof(cornerPreference)),
	)

	// 刷新窗体尺寸与框架 (920 x 650)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 920, 650, uintptr(SWP_NOMOVE|SWP_NOZORDER|SWP_FRAMECHANGED))

	// 启用 DWM 硬件柔和阴影
	margins := MARGINS{1, 1, 1, 1}
	procDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&margins)))

	// 绑定原生平滑拖拽事件
	w.Bind("goDragWindow", func() error {
		procReleaseCapture.Call()
		procSendMessage.Call(hwnd, uintptr(WM_NCLBUTTONDOWN), uintptr(HTCAPTION), 0)
		return nil
	})

	// 绑定后端 IPC 接口：查询端口
	w.Bind("goQueryPorts", func() (string, error) {
		ports, err := QueryAllPorts()
		if err != nil {
			return "[]", nil
		}
		data, _ := json.Marshal(ports)
		return string(data), nil
	})

	// 绑定后端 IPC 接口：强杀进程
	w.Bind("goKillProcess", func(pid uint32) (string, error) {
		ok, msg := KillProcessTree(pid)
		resp := map[string]interface{}{
			"success": ok,
			"message": msg,
		}
		data, _ := json.Marshal(resp)
		return string(data), nil
	})

	// 绑定窗口控制按钮
	w.Bind("goMinimizeWindow", func() error {
		procShowWindow.Call(hwnd, uintptr(SW_MINIMIZE))
		return nil
	})

	isMaximized := false
	w.Bind("goMaximizeWindow", func() error {
		if !isMaximized {
			procShowWindow.Call(hwnd, uintptr(SW_MAXIMIZE))
			isMaximized = true
		} else {
			procShowWindow.Call(hwnd, uintptr(SW_RESTORE))
			isMaximized = false
		}
		return nil
	})

	w.Bind("goCloseWindow", func() error {
		w.Terminate()
		os.Exit(0)
		return nil
	})

	// 加载内嵌的苹果瓷白前端 UI
	w.SetHtml(htmlContent)

	// 渲染完毕后，优雅展现 920x650 最终大小的圆角窗体
	procShowWindow.Call(hwnd, uintptr(SW_SHOW))

	w.Run()
}
