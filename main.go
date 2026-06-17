package main

import (
	"context"
	"embed"
	"os"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/guiapp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if !guiapp.EnsureCoreServiceRunning() {
		println("警告：无法启动核心服务，GUI 将以有限功能模式运行")
	}

	app := NewApp()

	windowStartState := options.Normal
	for _, arg := range os.Args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			windowStartState = options.Minimised
			break
		}
	}

	// 创建应用
	err := wails.Run(&options.App{
		Title:            appmeta.AppName,
		Width:            1024,
		Height:           768,
		Frameless:        guiapp.DefaultFrameless(),
		WindowStartState: windowStartState,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		OnStartup: func(ctx context.Context) {
			guiapp.SetWailsContext(ctx)
			app.Startup(ctx)
		},
		OnBeforeClose: app.OnWindowClosing,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "d2111a29-a967-4e46-807f-2fb5fcff9ed4-gui",
			OnSecondInstanceLaunch: guiapp.OnSecondInstanceLaunch,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Windows:          guiapp.ResolveWindowsOptions(),
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
