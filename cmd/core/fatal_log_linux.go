//go:build linux

package main

import (
	"os"
	"path/filepath"
	"time"
)

func setupFatalOutput() (func(), string) {
	exePath, err := os.Executable()
	if err != nil {
		return func() {}, ""
	}

	logDir := filepath.Join(filepath.Dir(exePath), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return func() {}, ""
	}

	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+"-core.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return func() {}, ""
	}

	oldStderr := os.Stderr
	oldStdout := os.Stdout
	os.Stderr = f
	os.Stdout = f

	return func() {
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		f.Close()
	}, logFile
}
