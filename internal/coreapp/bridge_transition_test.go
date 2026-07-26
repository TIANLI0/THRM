package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/types"
)

// TestStartingBridgeIsNotTreatedAsFailure 覆盖 #6 引入的过渡态语义。
//
// 桥接启动握手改到后台执行后，启动窗口内的读取必然失败。若把这个过渡态当成故障，
// 自愈逻辑会反复杀掉正在启动的进程，形成永不收敛的重启风暴。
func TestStartingBridgeIsNotTreatedAsFailure(t *testing.T) {
	starting := types.TemperatureData{
		BridgeOk:  false,
		BridgeMsg: bridge.StartingMessage,
	}
	if shouldRestartTemperatureBridge(starting) {
		t.Fatal("桥接启动过渡态被判定为需要重启，会造成重启风暴")
	}

	// 真实故障仍必须触发自愈。
	failures := []string{
		"桥接程序通信失败: 读取 GetTemperature 响应超时 (timeout=10s)",
		"启动桥接程序失败: THRM TempBridge 不存在",
		"桥接程序未连接",
		"broken pipe",
		"",
	}
	for _, msg := range failures {
		if !shouldRestartTemperatureBridge(types.TemperatureData{BridgeOk: false, BridgeMsg: msg}) {
			t.Errorf("真实故障未触发自愈: %q", msg)
		}
	}

	// 桥接正常时永不重启。
	if shouldRestartTemperatureBridge(types.TemperatureData{BridgeOk: true}) {
		t.Fatal("桥接正常时不应重启")
	}
}

// TestIdleSamplingIntervals 覆盖空闲判据：只有"无 GUI 且未开智能控温"才降频。
// 同一判据也用于放慢 HID 读循环的空转轮询。
func TestIdleSamplingIntervals(t *testing.T) {
	tests := []struct {
		name        string
		rate        int
		hasClients  bool
		autoControl bool
		wantIdle    bool
	}{
		{name: "有 GUI 时保持配置频率", rate: 2, hasClients: true, autoControl: false, wantIdle: false},
		{name: "开启智能控温时保持配置频率", rate: 2, hasClients: false, autoControl: true, wantIdle: false},
		{name: "两者都开时保持配置频率", rate: 2, hasClients: true, autoControl: true, wantIdle: false},
		{name: "完全空闲时降频", rate: 2, hasClients: false, autoControl: false, wantIdle: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveTemperatureMonitorInterval(tt.rate, tt.hasClients, tt.autoControl)
			if tt.wantIdle && got != idleTemperatureMonitorInterval {
				t.Fatalf("间隔 = %s, want %s", got, idleTemperatureMonitorInterval)
			}
			if !tt.wantIdle && got != temperatureMonitorInterval(tt.rate) {
				t.Fatalf("间隔 = %s, want %s", got, temperatureMonitorInterval(tt.rate))
			}
		})
	}
}
