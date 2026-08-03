//go:build windows

package coreapp

import (
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

func temperatureHistoryPath(installDir string) string {
	return filepath.Join(installDir, temperature.DefaultHistoryRelativePath)
}

func migrateLegacyTemperatureHistory(string, string, types.Logger) {}
