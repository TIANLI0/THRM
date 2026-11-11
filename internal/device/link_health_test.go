package device

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/sstallion/go-hid"
)

func TestLinkHealthRequiresFreshValidatedRX(t *testing.T) {
	var h hidLinkHealth
	h.reset(1)
	now := time.Now()
	frame := deviceproto.BuildFrame(deviceproto.CmdQueryWorkMode, 5)
	h.record(1, true, frame, now)
	if h.alive(now) {
		t.Fatal("TX alone must not prove liveness")
	}
	bad := append([]byte(nil), frame...)
	bad[len(bad)-1]++
	h.record(1, false, bad, now)
	if h.alive(now) {
		t.Fatal("invalid RX must not prove liveness")
	}
	h.record(1, false, frame, now)
	if !h.alive(now) {
		t.Fatal("valid response must prove liveness")
	}
	if h.alive(now.Add(hidLinkAliveWindow + time.Millisecond)) {
		t.Fatal("stale response must expire")
	}
	h.reset(2)
	h.record(1, false, frame, now)
	if h.alive(now) {
		t.Fatal("detached reader must not mark replacement link alive")
	}
}

func TestLinkRecorderRetainsCommandsAcrossHoursOfStatusReports(t *testing.T) {
	var h hidLinkHealth
	h.reset(7)
	now := time.Now()
	h.record(7, true, deviceproto.BuildFrame(deviceproto.CmdSetRealtimeRPM, 0xd0, 7), now)
	status := deviceproto.BuildFrame(deviceproto.CmdStatusNotify, 0x2a, 5, 0, 0xd0, 7, 0xd0, 7)
	for i := range 100000 {
		h.record(7, false, status, now.Add(time.Duration(i)*time.Second))
	}
	var snapshot struct {
		RXCount uint64 `json:"rxCount"`
		Recent  []struct {
			Direction string `json:"direction"`
			Raw       string `json:"raw"`
		} `json:"recent"`
	}
	if err := json.Unmarshal([]byte(h.snapshot(now.Add(100000*time.Second))), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.RXCount != 100000 || len(snapshot.Recent) != 1 || snapshot.Recent[0].Direction != "tx" {
		t.Fatalf("status reports displaced the last command: %+v", snapshot)
	}
	for range 100 {
		h.record(7, true, []byte{1, 2, 3}, now)
	}
	if err := json.Unmarshal([]byte(h.snapshot(now)), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Recent) != 32 {
		t.Fatalf("recorder must stay bounded, got %d events", len(snapshot.Recent))
	}
}

func TestRealtimeRecoveryKeepsResponsiveDevice(t *testing.T) {
	for _, tc := range []struct{ probe, recent, disconnect bool }{
		{true, true, false}, {true, false, false}, {false, true, false}, {false, false, true},
	} {
		if got := realtimeRecoveryNeedsDisconnect(tc.probe, tc.recent); got != tc.disconnect {
			t.Fatalf("probe=%t recent=%t: disconnect=%t", tc.probe, tc.recent, got)
		}
	}
}

func TestStaleRecoveryCannotCloseReplacementConnection(t *testing.T) {
	m := NewManager(testLogger{})
	m.isConnected = true
	m.device = &hid.Device{} // must never be used or closed in this test
	m.connectionGen.Store(2)
	m.realtimeWriteRecoveryScheduled = true
	m.disconnectGeneration(true, false, 1)
	if !m.isConnected || m.device == nil {
		t.Fatal("stale recovery closed the replacement connection")
	}
	m.realtimeWriteRecoveryScheduled = false // a successful command cancelled recovery
	m.disconnectGeneration(true, false, 2)
	if !m.isConnected || m.device == nil {
		t.Fatal("cancelled recovery closed a responsive connection")
	}
}

func TestPendingWindowsWriteCannotReuseNativeHandle(t *testing.T) {
	err := errors.New("hid_write/WaitForSingleObject: (0x000003E5) 重叠 I/O 操作在进行中。")
	if !isPendingHIDWriteFailure(err) || isPendingHIDWriteFailure(errors.New("等待命令 0x21 响应超时")) || isPendingHIDWriteFailure(nil) {
		t.Fatal("native pending I/O must be distinguished from protocol ACK timeout")
	}
	m := NewManager(testLogger{})
	m.device = &hid.Device{} // a poisoned handle must never reach its native Write
	m.hidWritePoisoned = err
	if _, got := m.writeHIDReportLocked(deviceproto.BuildFrame(deviceproto.CmdQueryWorkMode), hidControlReportLen); !errors.Is(got, err) {
		t.Fatalf("poisoned handle wasn't quarantined: %v", got)
	}
}
