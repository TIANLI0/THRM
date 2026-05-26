package main

import (
	"context"
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/appmeta"
	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

//go:embed all:frontend/dist
var assets embed.FS

var mainLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	mainLogger = logger.Sugar()
}

var wailsContext *context.Context
var ensureCoreServiceRunningMu sync.Mutex

// onSecondInstanceLaunch 当第二个实例启动时的回调函数
func onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	println("检测到第二个实例启动，参数:", strings.Join(secondInstanceData.Args, ","))
	println("工作目录:", secondInstanceData.WorkingDirectory)

	if wailsContext != nil {
		wailsruntime.WindowUnminimise(*wailsContext)
		wailsruntime.WindowShow(*wailsContext)
		wailsruntime.WindowSetAlwaysOnTop(*wailsContext, true)
		go func() {
			time.Sleep(1 * time.Second)
			wailsruntime.WindowSetAlwaysOnTop(*wailsContext, false)
		}()

		wailsruntime.EventsEmit(*wailsContext, "secondInstanceLaunch", secondInstanceData.Args)
	}
}

// ensureCoreServiceRunning 确保核心服务正在运行
func ensureCoreServiceRunning() bool {
	ensureCoreServiceRunningMu.Lock()
	defer ensureCoreServiceRunningMu.Unlock()

	// 检测是否在 Wails 绑定生成模式下运行
	exePath, err := os.Executable()
	if err == nil {
		tempDir := os.TempDir()
		if strings.HasPrefix(exePath, tempDir) {
			mainLogger.Info("检测到绑定生成模式，跳过核心服务启动")
			return true
		}
	}

	// 检查核心服务是否已经在运行
	if ipc.CheckCoreServiceRunning() {
		mainLogger.Info("核心服务已经在运行")
		return true
	}

	mainLogger.Info("核心服务未运行，正在启动...")

	// 获取核心服务路径
	if err != nil {
		mainLogger.Errorf("获取可执行文件路径失败: %v", err)
		return false
	}

	exeDir := filepath.Dir(exePath)
	corePath := appmeta.FirstExistingPath(appmeta.CoreExecutableCandidates(exeDir))

	// 检查核心服务是否存在
	if corePath == "" {
		mainLogger.Errorf("核心服务程序不存在: %v", appmeta.CoreExecutableCandidates(exeDir))
		return false
	}

	// 启动核心服务
	cmd := exec.Command(corePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	}

	if err := cmd.Start(); err != nil {
		mainLogger.Errorf("启动核心服务失败: %v", err)
		return false
	}

	mainLogger.Infof("核心服务已启动，PID: %d", cmd.Process.Pid)

	// 释放进程句柄
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	// 等待核心服务就绪
	for range 50 {
		time.Sleep(100 * time.Millisecond)
		if ipc.CheckCoreServiceRunning() {
			mainLogger.Info("核心服务已就绪")
			return true
		}
	}

	mainLogger.Warn("等待核心服务就绪超时")
	return false
}

func main() {
	if !ensureCoreServiceRunning() {
		mainLogger.Warn("警告：无法启动核心服务，GUI 将以有限功能模式运行")
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
		Frameless:        goruntime.GOOS == "windows",
		WindowStartState: windowStartState,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		OnStartup: func(ctx context.Context) {
			wailsContext = &ctx
			app.startup(ctx)
		},
		OnBeforeClose: app.OnWindowClosing,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "d2111a29-a967-4e46-807f-2fb5fcff9ed4-gui",
			OnSecondInstanceLaunch: onSecondInstanceLaunch,
		},
		Windows: &windows.Options{
			WindowIsTranslucent: true,
		},
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
