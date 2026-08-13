package autostart

import "testing"

// TestIsInstallAutoStartRequest 覆盖安装器专用开关的识别。
//
// 关键是不能把 --autostart（真正的自启动运行）误判成安装模式：那会让核心每次开机
// 只重写一遍自启动配置就退出，风扇彻底不受控。
func TestIsInstallAutoStartRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"无参数", nil, false},
		{"空切片", []string{}, false},
		{"双横线", []string{"--install-autostart"}, true},
		{"斜杠", []string{"/install-autostart"}, true},
		{"单横线", []string{"-install-autostart"}, true},
		{"混在其它参数中", []string{"--debug", "--install-autostart"}, true},
		{"自启动运行不是安装", []string{"--autostart"}, false},
		{"调试不是安装", []string{"--debug"}, false},
		{"前缀相近但不相等", []string{"--install-autostart-now"}, false},
		{"缺少前导符不算命中", []string{"install-autostart"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInstallAutoStartRequest(tt.args); got != tt.want {
				t.Fatalf("IsInstallAutoStartRequest(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestInstallAutoStartAndAutoStartAreDisjoint 两个开关的判定必须互不干扰：
// DetectAutoStartLaunch 命中的参数不应被当成安装请求，反之亦然。
func TestInstallAutoStartAndAutoStartAreDisjoint(t *testing.T) {
	for _, arg := range []string{"--autostart", "/autostart", "-autostart"} {
		if IsInstallAutoStartRequest([]string{arg}) {
			t.Fatalf("%q 被误判为安装请求", arg)
		}
	}

	for _, arg := range []string{"--install-autostart", "/install-autostart", "-install-autostart"} {
		if !IsInstallAutoStartRequest([]string{arg}) {
			t.Fatalf("%q 未被识别为安装请求", arg)
		}
	}
}
