//go:build windows

package ipc

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// pipeSecurityDescriptor 决定谁能连上 Core 的命名管道。
//
// Core 能改风扇、装驱动、刷固件，管道不能对本机所有进程开放（旧值 D:P(A;;GA;;;WD) 即 Everyone）。
//   - Core 已提权（管理员账户，计划任务 HighestAvailable 或由 GUI 拉起）：只允许 SYSTEM 与
//     Administrators。UAC 下未提权进程令牌里的 Administrators 组是 deny-only，连不上；
//     GUI 的清单要求管理员权限，始终满足。
//   - Core 未提权（标准用户账户自启动、开发环境）：再加上运行 Core 的用户自己，
//     否则同一用户下未提权的客户端无法连接；这种情况下 Core 本身也没有可被借用的高权限。
func pipeSecurityDescriptor() string {
	const elevatedOnly = "D:P(A;;GA;;;SY)(A;;GA;;;BA)"
	token := windows.GetCurrentProcessToken()
	if token.IsElevated() {
		return elevatedOnly
	}
	user, err := token.GetTokenUser()
	if err != nil || user.User.Sid == nil {
		// 取不到当前用户时退到"本机交互式登录的用户"，仍比 Everyone 窄。
		return elevatedOnly + "(A;;GA;;;IU)"
	}
	return elevatedOnly + "(A;;GA;;;" + user.User.Sid.String() + ")"
}

func listenIPC() (net.Listener, string, error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor(),
	}

	addr := ipcEndpointFromName(activePipeName())
	listener, err := winio.ListenPipe(addr, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("创建命名管道失败: %v", err)
	}
	return listener, addr, nil
}

func dialIPC(endpoint string, timeout time.Duration) (net.Conn, error) {
	t := timeout
	return winio.DialPipe(endpoint, &t)
}

func ipcEndpointFromName(name string) string {
	return `\\.\pipe\` + name
}
