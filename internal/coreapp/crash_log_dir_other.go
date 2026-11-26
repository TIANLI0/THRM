//go:build !linux && !windows && !darwin

package coreapp

func platformCrashLogDir() string {
	return ""
}
