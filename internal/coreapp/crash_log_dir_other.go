//go:build !linux && !windows

package coreapp

func platformCrashLogDir() string {
	return ""
}
