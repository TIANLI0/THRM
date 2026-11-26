//go:build darwin

package coreapp

import (
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

// temperatureHistoryPath 把温度历史放在用户状态目录：
// ~/Library/Application Support/.thrm/state/temperature-history.json
//
// 之前 darwin 走的是 history_path_other.go 的空实现，返回空串，
// 结果温度历史图表永远没数据、也不落盘。
func temperatureHistoryPath(string) string {
	homeDir, _ := os.UserHomeDir()
	stateDir := appmeta.UserStateDir(homeDir)
	if stateDir == "" {
		return ""
	}
	return filepath.Join(stateDir, temperature.DefaultHistoryRelativePath)
}

// migrateLegacyTemperatureHistory 在本平台上没有历史版本可迁移。
func migrateLegacyTemperatureHistory(string, string, types.Logger) {}
