package device

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

// hresultError 复刻 go-ole 的 *OleError：把 HRESULT 包成 error 并用 Code() 暴露。
type hresultError struct{ code uintptr }

func (e hresultError) Error() string { return fmt.Sprintf("HRESULT 0x%08X", e.code) }
func (e hresultError) Code() uintptr { return e.code }

// tinygo 的 Windows 适配器 Enable() 就是 ole.RoInitialize(1)，而 RoInitialize 在
// 当前 OS 线程已初始化过 COM 时返回 S_FALSE(1)——成功语义被 go-ole 包成了 error。
// 协程不绑定 OS 线程，误判成失败的结果就是唤醒后 BS1 随机连不上（issue #43）。
func TestIsAdapterEnabledAcceptsAlreadyInitialized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil 是成功", nil, true},
		{"S_FALSE 表示本线程已初始化", hresultError{0x00000001}, true},
		{"RPC_E_CHANGED_MODE 同样可继续使用", hresultError{0x80010106}, true},
		{"真正的失败仍然是失败", hresultError{0x80004005}, false},
		{"不带 HRESULT 的错误按失败处理", errors.New("adaptor is not powered"), false},
		{"包装后的 S_FALSE 也要能识别", fmt.Errorf("enable: %w", hresultError{0x00000001}), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAdapterEnabled(tc.err); got != tc.want {
				t.Fatalf("isAdapterEnabled(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// Connect 不能因为 Enable() 返回 S_FALSE 就直接放弃，它必须照常进入扫描。
func TestBLEConnectScansWhenAdapterAlreadyInitialized(t *testing.T) {
	adapter := newBlockingBLEAdapter()
	manager := NewBLEManager(nil)
	manager.adapter = alreadyInitializedAdapter{adapter}
	manager.scanTimeout = 20 * time.Millisecond

	done := make(chan struct{})
	go func() {
		manager.Connect()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Connect 在 Enable 返回 S_FALSE 后没有推进到扫描")
	}

	select {
	case <-adapter.stop:
	default:
		t.Fatal("Enable 返回 S_FALSE 时 Connect 直接返回了，从未发起扫描")
	}
}

type alreadyInitializedAdapter struct{ *blockingBLEAdapter }

func (alreadyInitializedAdapter) Enable() error { return hresultError{0x00000001} }

// 0x23 的语义是"进入实时转速模式"，不是无副作用的保活包。挡位模式下周期性发它，
// 会把固件踢回实时模式（此时目标转速是 0），风扇随即停转——这正是 issue #44。
func TestBS1KeepAliveNeverLeavesGearMode(t *testing.T) {
	for i := range 8 {
		got := bs1KeepAliveCommand(false, i)
		if !bytes.Equal(got, types.BS1CmdKeepAlive) {
			t.Fatalf("挡位模式第 %d 轮保活发出 %#x，只允许发 0x45 状态查询", i, got)
		}
	}

	var sawRealtime bool
	for i := range 8 {
		if bytes.Equal(bs1KeepAliveCommand(true, i), types.BS1CmdEnterRealtime) {
			sawRealtime = true
		}
	}
	if !sawRealtime {
		t.Fatal("实时模式下保活从未发出 0x23，实时会话得不到续期")
	}
}

// 新建的管理器把已连接状态和写入替身装好，便于观察实际发出的命令序列。
func newFakeBS1(t *testing.T) (*BLEManager, *[][]byte) {
	t.Helper()
	manager := NewBLEManager(nil)
	sent := &[][]byte{}
	manager.isConnected = true
	manager.writeFrame = func(cmd []byte) error {
		*sent = append(*sent, bytes.Clone(cmd))
		return nil
	}
	return manager, sent
}

func commandBytes(t *testing.T, frames [][]byte) []byte {
	t.Helper()
	out := make([]byte, 0, len(frames))
	for _, frame := range frames {
		parsed, ok := deviceproto.ParseFrame(frame)
		if !ok {
			t.Fatalf("发出的帧无法解析: %#x", frame)
		}
		out = append(out, parsed.Command)
	}
	return out
}

func TestBS1SetManualGearClearsRealtimeMode(t *testing.T) {
	manager, sent := newFakeBS1(t)
	manager.realtimeMode = true

	if err := manager.SetManualGear("标准"); err != nil {
		t.Fatalf("SetManualGear: %v", err)
	}

	if got := commandBytes(t, *sent); len(got) != 1 || got[0] != deviceproto.CmdSetFixedGear {
		t.Fatalf("切挡发出的命令序列 = %#x，期望只有一条 0x08", got)
	}
	if manager.realtimeMode {
		t.Fatal("切挡后 realtimeMode 仍为 true，保活包会继续发 0x23 把挡位打回实时模式")
	}
}

// 进入实时模式的前置命令必须是 0x23。此前发的是 0x46 0x01（RGB 使能），
// 与转速毫无关系；而真正的 0x23 被当成了保活包，两者位置写反了。
func TestBS1SetFanSpeedEntersRealtimeOnce(t *testing.T) {
	manager, sent := newFakeBS1(t)

	if err := manager.SetFanSpeed(2000); err != nil {
		t.Fatalf("SetFanSpeed: %v", err)
	}
	want := []byte{deviceproto.CmdEnterRealtimeRPM, deviceproto.CmdSetRealtimeRPM}
	if got := commandBytes(t, *sent); !bytes.Equal(got, want) {
		t.Fatalf("首次设置转速的命令序列 = %#x，期望 %#x", got, want)
	}
	if !manager.realtimeMode {
		t.Fatal("设置转速后 realtimeMode 应为 true")
	}

	// 已经在实时模式里，后续写入不该重复握手，也不该再付出一次模式切换等待。
	*sent = nil
	if err := manager.SetFanSpeed(2400); err != nil {
		t.Fatalf("SetFanSpeed: %v", err)
	}
	if got := commandBytes(t, *sent); len(got) != 1 || got[0] != deviceproto.CmdSetRealtimeRPM {
		t.Fatalf("后续设置转速的命令序列 = %#x，期望只有一条 0x21", got)
	}
}

func TestBS1SetFanSpeedRejectsOutOfRange(t *testing.T) {
	manager, sent := newFakeBS1(t)

	// Manager.SetFanSpeed 的范围校验写在 BS1 分支之后，BS1 必须自己拦住越界值。
	for _, rpm := range []int{-1, types.RealtimeRPMMax + 1} {
		if err := manager.SetFanSpeed(rpm); err == nil {
			t.Fatalf("SetFanSpeed(%d) 应当被拒绝", rpm)
		}
	}
	if len(*sent) != 0 {
		t.Fatalf("越界转速不应下发任何命令，实际发出 %d 帧", len(*sent))
	}
}

// 写入失败后固件模式未知，下次必须重新走一遍模式握手。
func TestBS1SetFanSpeedResetsModeAfterWriteFailure(t *testing.T) {
	manager, _ := newFakeBS1(t)
	manager.realtimeMode = true
	manager.writeFrame = func([]byte) error { return errors.New("链路断了") }

	if err := manager.SetFanSpeed(2000); err == nil {
		t.Fatal("写入失败时 SetFanSpeed 应当返回错误")
	}
	if manager.realtimeMode {
		t.Fatal("目标写入失败后不能继续假定固件仍在实时模式")
	}
}
