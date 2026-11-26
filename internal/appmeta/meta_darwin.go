//go:build darwin

package appmeta

import (
	"os"
	"path/filepath"
)

// macOS 上 GUI 与核心一起放在 THRM.app/Contents/MacOS 下，GUI 可执行文件名与
// .app 同名（Wails 的约定），核心沿用 Linux 的小写命名。
const (
	ExecutableName       = "THRM"
	LegacyExecutableName = "thrm"
	CoreExecutableName   = "thrm-core"
	LegacyCoreExecutable = "bs2pro-core"
	BridgeName           = ""
	BridgeExecutableName = ""
	PawnIOInstallerName  = ""

	// BundleIdentifier 与 build/config.yml 的 productIdentifier 保持一致。
	BundleIdentifier = "top.tianli0.thrm"
)

// UserConfigDir 遵循 macOS 惯例：~/Library/Application Support/THRM。
func UserConfigDir(homeDir string) string {
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, "Library", "Application Support", AppName)
}

// UserStateDir 与配置同目录：macOS 没有 XDG_STATE_HOME 的对应物，
// 温度历史等数据放 Application Support 即可被 Time Machine 备份。
func UserStateDir(homeDir string) string {
	return UserConfigDir(homeDir)
}

// UserLogDir 是 ~/Library/Logs/THRM，“控制台”应用会自动收录这里的日志。
func UserLogDir(homeDir string) string {
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, "Library", "Logs", AppName)
}

func CoreExecutableCandidates(baseDir string) []string {
	candidates := []string{
		filepath.Join(baseDir, CoreExecutableName),
		filepath.Join(baseDir, LegacyCoreExecutable),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, CoreExecutableName),
			filepath.Join(exeDir, LegacyCoreExecutable),
			filepath.Join(exeDir, "..", "Helpers", CoreExecutableName),
		)
	}
	return candidates
}

func GUIExecutableCandidates(baseDir string) []string {
	candidates := []string{
		filepath.Join(baseDir, ExecutableName),
		filepath.Join(baseDir, LegacyExecutableName),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, ExecutableName),
			filepath.Join(exeDir, LegacyExecutableName),
			// 开发时 GUI 在 build/bin/THRM.app 里，核心在 build/bin 下。
			filepath.Join(exeDir, ExecutableName+".app", "Contents", "MacOS", ExecutableName),
		)
	}
	return candidates
}

func BridgeExecutableCandidates(baseDir string) []string {
	return nil
}

func PawnIOInstallerPath(baseDir string) string {
	return ""
}

func PawnIOInstallerCandidates(baseDir string) []string {
	return nil
}
