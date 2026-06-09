//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/TIANLI0/THRM/internal/config"
	"golang.org/x/sys/windows"
)

func setupFatalOutput() (func(), string) {
	_ = os.Setenv("GOTRACEBACK", "all")
	debug.SetTraceback("all")

	installDir := config.GetInstallDir()
	if installDir == "" {
		installDir = "."
	}
	logDir := filepath.Join(installDir, "logs")
	_ = os.MkdirAll(logDir, 0755)
	filePath := filepath.Join(logDir, fmt.Sprintf("fatal_%s.log", time.Now().Format("2006-01-02_15-04-05.000")))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return func() {}, ""
	}

	h := windows.Handle(f.Fd())
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, h)
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, h)
	os.Stderr = f
	os.Stdout = f

	return func() { _ = f.Close() }, filePath
}
