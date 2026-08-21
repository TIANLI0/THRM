package device

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

func validResponse(command byte, payload ...byte) deviceproto.Frame {
	raw := deviceproto.BuildFrame(command, payload...)
	frame, _ := deviceproto.ParseFrame(raw)
	return frame
}

func TestResponseBrokerRoutesExactCommand(t *testing.T) {
	broker := newResponseBroker()
	firmware := broker.register(deviceproto.CmdQueryFirmwareVersion)
	workMode := broker.register(deviceproto.CmdQueryWorkMode)

	if !broker.deliver(validResponse(deviceproto.CmdQueryWorkMode, 5)) {
		t.Fatal("work-mode response was not delivered")
	}
	select {
	case frame := <-workMode.result:
		if frame.Command != deviceproto.CmdQueryWorkMode {
			t.Fatalf("wrong command: 0x%02X", frame.Command)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for routed response")
	}
	select {
	case <-firmware.result:
		t.Fatal("response leaked to a waiter for another command")
	default:
	}
}

func TestResponseBrokerRejectsBadChecksum(t *testing.T) {
	broker := newResponseBroker()
	waiter := broker.register(deviceproto.CmdQueryFirmwareVersion)
	frame := validResponse(deviceproto.CmdQueryFirmwareVersion, 0, 0, 3, 5)
	frame.ChecksumOK = false
	if broker.deliver(frame) {
		t.Fatal("bad-checksum frame was delivered")
	}
	broker.cancel(waiter)
}

func TestValidateACK(t *testing.T) {
	if err := validateACK(validResponse(deviceproto.CmdEnterRealtimeRPM, 3), 1, 3); err != nil {
		t.Fatalf("accepted idempotent ACK rejected: %v", err)
	}
	if err := validateACK(validResponse(deviceproto.CmdSetRealtimeRPM, 2), 1); err == nil {
		t.Fatal("firmware rejection was accepted")
	}
}

func TestParseFanDataRequiresValidDeclaredFrameAndChecksum(t *testing.T) {
	manager := &Manager{}
	frame := deviceproto.BuildFrame(deviceproto.CmdStatusNotify,
		0x2A, 0x05, 0x00, 0xD0, 0x07, 0xE8, 0x03)
	report := deviceproto.BuildReport(frame, hidControlReportLen)
	data := manager.parseFanData(report, len(report))
	if data == nil || data.FrameLength != 9 || data.CurrentRPM != 2000 || data.TargetRPM != 1000 {
		t.Fatalf("valid status frame decoded incorrectly: %#v", data)
	}

	corrupt := append([]byte(nil), report...)
	corrupt[len(frame)] ^= 0xFF // report ID shifts the frame checksum to index len(frame)
	if data := manager.parseFanData(corrupt, len(corrupt)); data != nil {
		t.Fatalf("checksum-corrupt notification was accepted: %#v", data)
	}
}

func TestDebugCaptureUsesRequestDecoderForTX(t *testing.T) {
	manager := &Manager{}
	manager.debugCapture.Store(true)
	report := deviceproto.BuildReport(deviceproto.BuildFrame(deviceproto.CmdQueryFirmwareVersion), hidControlReportLen)
	manager.recordDebugFrame("tx", "hid", report)
	frames := manager.debugFramesAfter(0)
	if len(frames) != 1 {
		t.Fatalf("captured frames = %d", len(frames))
	}
	if frames[0].Decoded != "request: query firmware version" {
		t.Fatalf("TX decode = %q", frames[0].Decoded)
	}
}

func TestFixedGearForWakePreservesReportedGear(t *testing.T) {
	for _, test := range []struct {
		gearByte byte
		want     byte
	}{
		{gearByte: 0x28, want: 1},
		{gearByte: 0x2a, want: 2},
		{gearByte: 0x4c, want: 3},
		{gearByte: 0x6e, want: 4},
		{gearByte: 0xff, want: 1},
	} {
		if got := fixedGearForWake(&types.FanData{GearSettings: test.gearByte}); got != test.want {
			t.Fatalf("fixedGearForWake(0x%02X) = %d, want %d", test.gearByte, got, test.want)
		}
	}
	if got := fixedGearForWake(nil); got != 1 {
		t.Fatalf("fixedGearForWake(nil) = %d, want 1", got)
	}
}
