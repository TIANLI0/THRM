package guiapp

import (
	goruntime "runtime"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// resolvedBlurEnabled 记录本次窗口创建时实际启用的模糊状态，供前端(WindowBlurEnabled)
// 决定背景透明度，确保与窗口的真实材质一致(模糊设置更改需重启才生效)。
var resolvedBlurEnabled bool

// ApplyWindowBackdrop 根据用户配置(windowBlur)与系统版本决定窗口材质，写进主窗口的创建参数。
//
//   - auto：Win11 且系统缩放 ≥ 150% 时使用云母，否则关闭（见 autoBackdropMinDPI）。
//   - on：兼容旧配置，继续使用云母。
//   - acrylic/mica/tabbed/off：使用对应材质。
//
// 材质只在窗口创建时生效，更改后需重启应用。系统材质是 Windows 专有的，
// 其他平台一律按关闭处理，前端随之使用不透明背景。
func ApplyWindowBackdrop(options *application.WebviewWindowOptions) {
	backdrop := application.None
	if goruntime.GOOS == "windows" {
		backdrop = backdropTypeForMode(resolveWindowBlurMode(), isWindows11(), systemDPI() >= autoBackdropMinDPI)
	}
	resolvedBlurEnabled = backdrop != application.None
	options.Windows.BackdropType = backdrop
	if resolvedBlurEnabled {
		// Translucent 让窗口与 WebView2 背景都透明，材质才能透出来。
		options.BackgroundType = application.BackgroundTypeTranslucent
	} else {
		options.BackgroundType = application.BackgroundTypeSolid
	}
}

// WindowBlurEnabled 返回本次窗口创建时实际启用的模糊(半透明材质)状态。
// 前端据此决定是否使用透明背景：关闭模糊时应回退为不透明背景。
func (a *App) WindowBlurEnabled() bool {
	return resolvedBlurEnabled
}

// autoBackdropMinDPI 是 auto 模式启用云母所需的最低系统 DPI（144 = 150% 缩放）。
//
// 材质要求 WebView2 背景透明，而 Chromium 在透明背景上画字只能用灰度抗锯齿，
// 用不上 ClearType 子像素抗锯齿。高分屏上看不出差别，1080p 这类 100%/125% 缩放的
// 屏幕上文字会明显发虚。所以 auto 只在高缩放下开材质；用户明确选了某种材质仍照办。
const autoBackdropMinDPI = 144

func backdropTypeForMode(mode string, windows11, highDPI bool) application.BackdropType {
	switch types.NormalizeWindowBlur(mode) {
	case types.WindowBlurAcrylic:
		return application.Acrylic
	case types.WindowBlurMica:
		return application.Mica
	case types.WindowBlurTabbed:
		return application.Tabbed
	case types.WindowBlurOn:
		return application.Mica
	case types.WindowBlurOff:
		return application.None
	default:
		if windows11 && highDPI {
			return application.Mica
		}
		return application.None
	}
}

// resolveWindowBlurMode 从磁盘上的配置文件读取窗口模糊设置。
// GUI 进程在创建主窗口之前调用，此时核心服务已确保启动并写入配置；
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
