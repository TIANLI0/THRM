package device

import (
	"fmt"

	"github.com/TIANLI0/THRM/internal/deviceproto"
)

// InitializeFirmwareController runs command 0x03. The command leaves the RPM
// table intact but its first successful execution selects fixed gear 1, so Core
// must replay the App-owned mode after this returns.
func (m *Manager) InitializeFirmwareController() error {
	return m.runFirmwareMaintenanceCommand(deviceproto.CmdInitializeController, 1, 5)
}

// ClearFirmwareInitializationLatch clears only controller_state+1. It does not
// leave realtime mode or clear the rest of the runtime controller structure.
func (m *Manager) ClearFirmwareInitializationLatch() error {
	return m.runFirmwareMaintenanceCommand(deviceproto.CmdClearInitializationLatch, 1)
}

// FactoryReinitializeFirmware runs command 0x06. The firmware resets the four
// hardware RPM slots to 1700/2400/3000/4000 and rebuilds runtime/RGB state.
func (m *Manager) FactoryReinitializeFirmware() error {
	return m.runFirmwareMaintenanceCommand(deviceproto.CmdFactoryReinitialize, 1)
}

func (m *Manager) runFirmwareMaintenanceCommand(command byte, accepted ...byte) error {
	if m.IsBS1() {
		return fmt.Errorf("BS1 不支持此固件维护命令")
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.device == nil {
		return fmt.Errorf("设备未连接")
	}
	if !supportsDetailedFirmwareProtocol(m.productID) {
		return fmt.Errorf("命令 0x%02X 仅在 BS2/BS2PRO/BS3/BS3PRO 同协议固件上启用", command)
	}
	defer m.beginTransaction()()

	if err := m.sendHIDAckLocked(command, nil, accepted...); err != nil {
		return err
	}
	m.resetRealtimeControlStateLocked()
	// 0x06 会重建固件的 LED/RGB 状态，0x03/0x05 也会改写运行期控制结构，
	// 缓存的"设备侧灯光开关值"在这之后不再可信。
	m.resetLightStateCacheLocked()
	// 0x06 还会把四个挡位槽重置回 1700/2400/3000/4000，挡位转速缓存同样作废。
	m.resetGearRPMCacheLocked()
	return nil
}
