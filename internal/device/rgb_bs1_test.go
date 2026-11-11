package device

import (
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

// 回归测试：BS1 的灯光命令在持锁路径里不能再调用 IsBS1()。
//
// IsBS1() 内部会走 GetDeviceType() → m.mutex.RLock()，而 SetLightStrip /
// SetRGBOff / sendLightCommandLocked 都已经持有 m.mutex 写锁。Go 的 RWMutex
// 不可重入，于是直接自死锁：IPC 层 12 秒后报"等待 IPC 响应超时"，
// 温度监控循环也会因为拿不到同一把锁而卡死。
//
// 这里注入一个不应答的写入替身，让命令走 900ms 应答超时——只要函数能返回
// 就说明没有死锁。
func TestBS1LightCommandsDoNotSelfDeadlock(t *testing.T) {
	newBS1Manager := func() *Manager {
		m := NewManager(nil)
		m.mutex.Lock()
		m.isConnected = true
		m.deviceType = types.DeviceTypeBLE
		m.mutex.Unlock()

		m.bleManager.mutex.Lock()
		m.bleManager.isConnected = true
		m.bleManager.writeFrame = func([]byte) error { return nil }
		m.bleManager.mutex.Unlock()
		return m
	}

	cases := []struct {
		name string
		call func(*Manager) error
	}{
		// 只用单条命令的模式：这里验证的是"持锁时不要调用会加锁的 IsBS1()"，
		// 走到的正是 sendLightCommandLocked。静态模式要发 19 帧，而替身不应答，
		// 叠加帧级重发后会拖到几十秒，没有额外覆盖价值。
		{"SetLightStrip(off)", func(m *Manager) error {
			return m.SetLightStrip(types.LightStripConfig{Mode: "off", Brightness: 100})
		}},
		{"SetRGBOff", func(m *Manager) error {
			if m.SetRGBOff() {
				return nil
			}
			return errRGBOffReportedFailure
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newBS1Manager()
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = tc.call(m) // 应答超时是预期的，这里只关心"会不会卡住"
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("调用在 5 秒内没有返回，疑似持锁调用 IsBS1() 造成的自死锁")
			}
		})
	}
}

// errRGBOffReportedFailure 只用于让上面的用例表保持统一的 error 返回。
var errRGBOffReportedFailure = &lightTestError{}

type lightTestError struct{}

func (*lightTestError) Error() string { return "SetRGBOff 返回 false" }

func TestBS1LostCommitAckDoesNotRepeatFlashWrite(t *testing.T) {
	m := NewManager(nil)
	m.isConnected, m.deviceType = true, types.DeviceTypeBLE
	m.bleManager.isConnected = true
	writes := 0
	m.bleManager.writeFrame = func(raw []byte) error {
		frame, ok := deviceproto.ParseFrame(raw)
		if !ok || frame.Command != deviceproto.CmdRGBCommit {
			t.Fatal("unexpected command")
		}
		writes++
		// A transport failure is enough to exercise the ambiguous commit case:
		// the host cannot tell whether firmware already applied the command.
		return errors.New("lost connection after write")
	}
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBCommit, 1); err == nil {
		t.Fatal("lost ACK was hidden")
	}
	if writes != 1 {
		t.Fatalf("commit wrote flash %d times", writes)
	}
}

func TestBS1DisconnectInvalidatesLightingState(t *testing.T) {
	for _, unexpected := range []bool{false, true} {
		m := NewManager(nil)
		m.isConnected, m.deviceType = true, types.DeviceTypeBLE
		m.hasRGBEnabled, m.rgbEnabled, m.hasLightConfig = true, true, true
		// The underlying BLE/HID link is already gone. No native transport is
		// needed to ensure a future connection cannot reuse this cached switch.
		if unexpected {
			m.handleBLELinkLost()
		} else {
			m.DisconnectSilently()
		}
		if m.hasRGBEnabled || m.hasLightConfig {
			t.Fatalf("unexpected=%t: stale BS1 light cache survived disconnect", unexpected)
		}
	}
}
