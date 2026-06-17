package guiapp

import (
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// resolvedBlurEnabled 记录本次窗口创建时实际启用的模糊状态，供前端(WindowBlurEnabled)
// 决定背景透明度，确保与窗口的真实材质一致(模糊设置更改需重启才生效)。
var resolvedBlurEnabled bool

// ResolveWindowsOptions 根据用户配置(windowBlur)与系统版本决定窗口的模糊(云母/亚克力)效果。
//
//   - on   : 始终启用模糊(透明 + 云母背景)。
//   - off  : 关闭模糊(不透明窗口)。
//   - auto : Win11 启用、Win10 关闭(模糊设置的默认值)。
//
// 该选项在 Wails 中只能于窗口创建时生效，更改后需重启应用。
func ResolveWindowsOptions() *windows.Options {
	resolvedBlurEnabled = blurEnabledForCurrentSystem(resolveWindowBlurMode())
	if resolvedBlurEnabled {
		return &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica,
		}
	}
	return &windows.Options{
		WebviewIsTransparent: false,
		WindowIsTranslucent:  false,
		BackdropType:         windows.None,
	}
}

// WindowBlurEnabled 返回本次窗口创建时实际启用的模糊(半透明材质)状态。
// 前端据此决定是否使用透明背景：关闭模糊时应回退为不透明背景。
func (a *App) WindowBlurEnabled() bool {
	return resolvedBlurEnabled
}

func blurEnabledForCurrentSystem(mode string) bool {
	switch mode {
	case types.WindowBlurOn:
		return true
	case types.WindowBlurOff:
		return false
	default: // auto: Win11 开启, Win10 关闭
		return isWindows11()
	}
}

// resolveWindowBlurMode 从磁盘上的配置文件读取窗口模糊设置。
// GUI 进程在 wails.Run 之前调用，此时核心服务已确保启动并写入配置；
// 读取失败时回退为 auto，并兜底任何 panic 不影响窗口创建。
func resolveWindowBlurMode() (mode string) {
	mode = types.WindowBlurAuto
	defer func() {
		if r := recover(); r != nil {
			mainLogger.Warnf("读取窗口模糊配置失败，回退为 auto: %v", r)
			mode = types.WindowBlurAuto
		}
	}()

	manager := config.NewManager(config.GetInstallDir(), nil)
	cfg := manager.Load(false)
	return types.NormalizeWindowBlur(cfg.WindowBlur)
}
