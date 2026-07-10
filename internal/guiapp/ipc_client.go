package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func mergeTemperatureMetadata(previous, incoming types.TemperatureData) types.TemperatureData {
	merged := incoming
	if merged.CpuSensors == nil {
		merged.CpuSensors = previous.CpuSensors
	}
	if merged.GpuSensors == nil {
		merged.GpuSensors = previous.GpuSensors
	}
	if merged.CpuPowerSensors == nil {
		merged.CpuPowerSensors = previous.CpuPowerSensors
	}
	if merged.GpuPowerSensors == nil {
		merged.GpuPowerSensors = previous.GpuPowerSensors
	}
	if merged.GpuDevices == nil {
		merged.GpuDevices = previous.GpuDevices
	}
	if merged.CpuModel == "" {
		merged.CpuModel = previous.CpuModel
	}
	if merged.GpuModel == "" {
		merged.GpuModel = previous.GpuModel
	}
	if merged.SelectedGpuDevice == "" {
		merged.SelectedGpuDevice = previous.SelectedGpuDevice
	}
	return merged
}

func coreServiceUnavailableMessage(detail string) string {
	if detail == "" {
		return fmt.Sprintf("核心服务不可用。请检查 %s 是否仍在安装目录中，或是否被安全软件隔离。", appmeta.CoreExecutableName)
	}
	return fmt.Sprintf("核心服务不可用：%s。请检查 %s 是否仍在安装目录中，或是否被安全软件隔离。", detail, appmeta.CoreExecutableName)
}

func (a *App) emitCoreServiceError(detail string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "core-service-error", coreServiceUnavailableMessage(detail))
}

func (a *App) emitCoreServiceOK() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "core-service-ok", nil)
}

// handleCoreEvent 处理核心服务推送的事件
func (a *App) handleCoreEvent(event ipc.Event) {
	if a.ctx == nil {
		return
	}

	switch event.Type {
	case ipc.EventFanDataUpdate:
		var fanData types.FanData
		if err := json.Unmarshal(event.Data, &fanData); err == nil {
			runtime.EventsEmit(a.ctx, "fan-data-update", fanData)
		}

	case ipc.EventTemperatureUpdate:
		var temp types.TemperatureData
		if err := json.Unmarshal(event.Data, &temp); err == nil {
			a.mutex.Lock()
			temp = mergeTemperatureMetadata(a.currentTemp, temp)
			a.currentTemp = temp
			a.mutex.Unlock()
			runtime.EventsEmit(a.ctx, "temperature-update", temp)
		}

	case ipc.EventTemperatureHistoryUpdate:
		var point types.TemperatureHistoryPoint
		if err := json.Unmarshal(event.Data, &point); err == nil {
			runtime.EventsEmit(a.ctx, "temperature-history-update", point)
		}

	case ipc.EventDeviceConnected:
		var deviceInfo map[string]string
		json.Unmarshal(event.Data, &deviceInfo)
		a.mutex.Lock()
		a.isConnected = true
		a.mutex.Unlock()
		runtime.EventsEmit(a.ctx, "device-connected", deviceInfo)

	case ipc.EventDeviceDisconnected:
		a.mutex.Lock()
		a.isConnected = false
		a.mutex.Unlock()
		runtime.EventsEmit(a.ctx, "device-disconnected", nil)

	case ipc.EventDeviceError:
		var errMsg string
		json.Unmarshal(event.Data, &errMsg)
		runtime.EventsEmit(a.ctx, "device-error", errMsg)

	case ipc.EventConfigUpdate:
		var cfg types.AppConfig
		if err := json.Unmarshal(event.Data, &cfg); err == nil {
			runtime.EventsEmit(a.ctx, "config-update", cfg)
		}

	case ipc.EventHotkeyTriggered:
		var payload map[string]any
		if err := json.Unmarshal(event.Data, &payload); err == nil {
			runtime.EventsEmit(a.ctx, "hotkey-triggered", payload)
		}

	case ipc.EventLegionPowerModeUpdate:
		var payload map[string]any
		if err := json.Unmarshal(event.Data, &payload); err == nil {
			runtime.EventsEmit(a.ctx, "legion-power-mode-update", payload)
		}

	case ipc.EventHealthPing:
		var timestamp int64
		json.Unmarshal(event.Data, &timestamp)
		runtime.EventsEmit(a.ctx, "health-ping", timestamp)

	case ipc.EventHeartbeat:
		var timestamp int64
		json.Unmarshal(event.Data, &timestamp)
		runtime.EventsEmit(a.ctx, "heartbeat", timestamp)

	case "show-window":
		a.ShowWindow()

	case "quit":
		a.QuitApp()
	}
}

// sendRequest 发送请求到核心服务
func (a *App) sendRequest(reqType ipc.RequestType, data any) (*ipc.Response, error) {
	if !a.ipcClient.IsConnected() {
		if !EnsureCoreServiceRunning() {
			err := fmt.Errorf("核心服务未运行且启动失败")
			a.emitCoreServiceError(err.Error())
			return nil, err
		}
		if err := a.ipcClient.Connect(); err != nil {
			wrapped := fmt.Errorf("未连接到核心服务: %v", err)
			a.emitCoreServiceError(wrapped.Error())
			return nil, wrapped
		}
		a.ipcClient.SetEventHandler(a.handleCoreEvent)
		a.emitCoreServiceOK()
	}

	resp, err := a.ipcClient.SendRequest(reqType, data)
	if err == nil {
		a.emitCoreServiceOK()
		return resp, nil
	}

	guiLogger.Warnf("IPC 请求失败，尝试重新连接核心服务后重试: %v", err)
	a.ipcClient.Close()
	if !EnsureCoreServiceRunning() {
		wrapped := fmt.Errorf("核心服务连接断开且重新启动失败: %v", err)
		a.emitCoreServiceError(wrapped.Error())
		return nil, wrapped
	}
	if connectErr := a.ipcClient.Connect(); connectErr != nil {
		wrapped := fmt.Errorf("重新连接核心服务失败: %v；原始错误: %v", connectErr, err)
		a.emitCoreServiceError(wrapped.Error())
		return nil, wrapped
	}
	a.ipcClient.SetEventHandler(a.handleCoreEvent)

	resp, err = a.ipcClient.SendRequest(reqType, data)
	if err != nil {
		a.emitCoreServiceError(err.Error())
		return nil, err
	}
	a.emitCoreServiceOK()
	return resp, nil
}
