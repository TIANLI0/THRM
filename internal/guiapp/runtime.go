package guiapp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

var mainLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	mainLogger = logger.Sugar()
}

var wailsContext *context.Context
var ensureCoreServiceRunningMu sync.Mutex

// SetWailsContext 保存当前 Wails 上下文，供单实例回调恢复窗口使用。
func SetWailsContext(ctx context.Context) {
	wailsContext = &ctx
}

// OnSecondInstanceLaunch 处理第二个 GUI 实例启动时的窗口恢复逻辑。
func OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
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

// EnsureCoreServiceRunning 确保核心服务正在运行。
func EnsureCoreServiceRunning() bool {
	ensureCoreServiceRunningMu.Lock()
	defer ensureCoreServiceRunningMu.Unlock()

	exePath, err := os.Executable()
	if err == nil {
		tempDir := os.TempDir()
		if strings.HasPrefix(exePath, tempDir) {
			mainLogger.Info("检测到绑定生成模式，跳过核心服务启动")
			return true
		}
	}

	if ipc.CheckCoreServiceRunning() {
		mainLogger.Info("核心服务已经在运行")
		return true
	}

	mainLogger.Info("核心服务未运行，正在启动...")
	if err != nil {
		mainLogger.Errorf("获取可执行文件路径失败: %v", err)
		return false
	}

	exeDir := filepath.Dir(exePath)
	corePath := appmeta.FirstExistingPath(appmeta.CoreExecutableCandidates(exeDir))
	if corePath == "" {
		mainLogger.Errorf("核心服务程序不存在: %v", appmeta.CoreExecutableCandidates(exeDir))
		return false
	}

	cmd := exec.Command(corePath)
	configureCoreCommand(cmd)

	if err := cmd.Start(); err != nil {
		mainLogger.Errorf("启动核心服务失败: %v", err)
		return false
	}

	mainLogger.Infof("核心服务已启动，PID: %d", cmd.Process.Pid)
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	for i := range 100 {
		time.Sleep(100 * time.Millisecond)
		if ipc.CheckCoreServiceRunning() {
			mainLogger.Infof("核心服务已就绪（等待 %d ms）", (i+1)*100)
			return true
		}
	}

	mainLogger.Warn("等待核心服务就绪超时（10秒）")
	return false
}

// DefaultFrameless reports whether the desktop window should use the custom Windows frame.
func DefaultFrameless() bool {
	return goruntime.GOOS == "windows"
}
