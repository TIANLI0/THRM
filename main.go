package main

import (
	"embed"
	"os"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/guiapp"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// appIcon 是 Linux 下的窗口/任务栏图标；Windows 的图标随 .syso 资源嵌进可执行文件，
// macOS 的 Dock 图标来自 .app 包里的 icons.icns。
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	if !guiapp.EnsureCoreServiceRunning() {
		println("警告：无法启动核心服务，GUI 将以有限功能模式运行")
	}

	// WebKitGTK can hit NVIDIA's Wayland explicit-sync path without setting an
	// acquire point. Scope the workaround to machines actually using NVIDIA and
	// apply it after starting the non-GPU core service.
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := os.Stat("/sys/module/nvidia"); err == nil {
			if _, configured := os.LookupEnv("__NV_DISABLE_EXPLICIT_SYNC"); !configured {
				_ = os.Setenv("__NV_DISABLE_EXPLICIT_SYNC", "1")
			}
		}
	}

	gui := guiapp.New(newThemeManager())

	app := application.New(application.Options{
		Name:        appmeta.AppName,
		Description: "THRM 桌面控制中心",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(gui),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "d2111a29-a967-4e46-807f-2fb5fcff9ed4-gui",
			OnSecondInstanceLaunch: gui.OnSecondInstanceLaunch,
		},
		Linux: application.LinuxOptions{
			ProgramName:   "thrm",
			ApplicationID: "top.tianli0.thrm",
		},
		Mac: application.MacOptions{
			// 与 Windows/Linux 一致：关掉主窗口即退出 GUI，后台控温与菜单栏图标由核心负责。
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// macOS：窗口被“隐藏到托盘”后点 Dock 图标，要把它重新显示出来。
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) {
		gui.ShowWindow()
	})

	windowOptions := application.WebviewWindowOptions{
		Name:      "main",
		Title:     appmeta.AppName,
		MinWidth:  800,
		MinHeight: 600,
		Frameless: guiapp.DefaultFrameless(),
		URL:       "/",
		Windows: application.WindowsWindow{
			// 用 DirectComposition 托管 WebView2，Wails 才能按前端标记的
			// --wails-non-client-region 做原生命中测试：自绘最大化按钮返回 HTMAXBUTTON，
			// 悬停时弹出 Win11 贴靠布局。WebView2 不支持时 Wails 自动退回窗口托管。
			WebView2CompositionHosting: true,
		},
		Linux: application.LinuxWindow{
			WebviewGpuPolicy: application.WebviewGpuPolicyAlways,
		},
	}
	// 上次记忆的窗口尺寸、位置与最大化状态（GUI 本地持久化，独立于 IPC 核心服务）。
	guiapp.LoadWindowState().ApplyTo(&windowOptions)
	guiapp.ApplyWindowBackdrop(&windowOptions)
	for _, arg := range os.Args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			// 自启动优先以最小化到托盘启动，覆盖记忆的窗口状态。
			windowOptions.StartState = application.WindowStateMinimised
			break
		}
	}

	gui.AttachWindow(app.Window.NewWithOptions(windowOptions))

	if err := app.Run(); err != nil {
		println("Error:", err.Error())
	}
}
