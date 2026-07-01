//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

func listenIPC() (net.Listener, string, error) {
	addr := ipcEndpointFromName(activePipeName())
	_ = os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		return nil, "", fmt.Errorf("创建 unix socket 失败: %v", err)
	}

	_ = os.Chmod(addr, 0600)
	return listener, addr, nil
}

func dialIPC(endpoint string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	return dialer.Dial("unix", endpoint)
}

func ipcEndpointFromName(name string) string {
	return filepath.Join(os.TempDir(), name+".sock")
}
