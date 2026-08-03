//go:build !linux && !windows

package coreapp

import "github.com/TIANLI0/THRM/internal/types"

func temperatureHistoryPath(string) string {
	return ""
}

func migrateLegacyTemperatureHistory(string, string, types.Logger) {}
