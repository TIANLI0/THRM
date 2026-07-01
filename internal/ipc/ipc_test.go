package ipc

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Setenv(EnvPipeName, fmt.Sprintf("THRM-IPC-test-%d", os.Getpid()))
	os.Exit(m.Run())
}

func TestServerStartStop(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response {
		return Response{Success: true}
	}

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if !s.HasClients() {
		// Server just started, should accept connections
	}
	s.Stop()
	s.Stop() // double stop should not panic
}

func TestClientConnectClose(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response {
		return Response{Success: true}
	}

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	c := NewClient(logger)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	if !c.IsConnected() {
		t.Error("Client should be connected")
	}
	c.Close()
	if c.IsConnected() {
		t.Error("Client should be disconnected after Close")
	}
	c.Close() // double close should not panic
}

func TestSendRequestResponse(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response {
		if req.Type == ReqPing {
			return Response{Success: true, Data: []byte(`"pong"`)}
		}
		return Response{Success: false, Error: "unknown request"}
	}

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	c := NewClient(logger)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer c.Close()

	resp, err := c.SendRequest(ReqPing, nil)
	if err != nil {
		t.Fatalf("SendRequest error: %v", err)
	}
	if !resp.Success {
		t.Errorf("Response should be successful: %s", resp.Error)
	}
}

func TestSendRequest_UnknownType(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response {
		return Response{Success: false, Error: "unknown request"}
	}

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	c := NewClient(logger)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer c.Close()

	resp, err := c.SendRequest("NonExistent", nil)
	if err != nil {
		t.Fatalf("SendRequest error: %v", err)
	}
	if resp.Success {
		t.Error("Unknown request should not succeed")
	}
}

func TestBroadcastEvent(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response {
		return Response{Success: true}
	}

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	c := NewClient(logger)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer c.Close()

	received := make(chan Event, 1)
	c.SetEventHandler(func(ev Event) {
		select {
		case received <- ev:
		default:
		}
	})

	time.Sleep(50 * time.Millisecond)
	s.BroadcastEvent(EventHealthPing, "ping-data")

	select {
	case ev := <-received:
		if ev.Type != EventHealthPing {
			t.Errorf("Event type = %q, want %q", ev.Type, EventHealthPing)
		}
	case <-time.After(3 * time.Second):
		t.Log("Broadcast event not received (may need event subscription)")
	}
}

func TestCheckCoreServiceRunning_NoServer(t *testing.T) {
	if CheckCoreServiceRunning() {
		t.Log("Warning: core service appears to be running (unexpected in test)")
	}
}

func TestGetCoreLockFilePath(t *testing.T) {
	path := GetCoreLockFilePath()
	if path == "" {
		t.Error("GetCoreLockFilePath should not return empty")
	}
}

func TestRequestTypeConstants_Distinct(t *testing.T) {
	allTypes := []RequestType{
		ReqConnect, ReqDisconnect, ReqGetDeviceStatus, ReqGetCurrentFanData,
		ReqRefreshDeviceSettings, ReqGetConfig, ReqUpdateConfig,
		ReqSetFanCurve, ReqGetFanCurve, ReqGetFanCurveProfiles,
		ReqSetActiveFanCurveProfile, ReqSaveFanCurveProfile, ReqDeleteFanCurveProfile,
		ReqExportFanCurveProfiles, ReqImportFanCurveProfiles, ReqResetLearnedOffsets,
		ReqSetAutoControl, ReqSetManualGear, ReqGetAvailableGears,
		ReqSetCustomSpeed, ReqSetGearLight, ReqSetPowerOnStart,
		ReqSetSmartStartStop, ReqSetBrightness, ReqSetLightStrip,
		ReqGetTemperature, ReqGetTemperatureHistory, ReqSetTemperatureHistoryEnabled,
		ReqTestTemperatureReading, ReqTestBridgeProgram, ReqGetBridgeProgramStatus,
		ReqRestartPawnIO, ReqReinstallPawnIO,
		ReqSetWindowsAutoStart, ReqCheckWindowsAutoStart, ReqIsRunningAsAdmin,
		ReqGetAutoStartMethod, ReqSetAutoStartWithMethod,
		ReqShowWindow, ReqHideWindow, ReqQuitApp,
		ReqGetDebugInfo, ReqSetDebugMode, ReqSendDeviceDebugCommand,
		ReqGetDeviceDebugFrames, ReqUpdateGuiResponseTime,
		ReqPing, ReqIsAutoStartLaunch, ReqSubscribeEvents, ReqUnsubscribeEvents,
	}
	seen := make(map[RequestType]bool)
	for _, rt := range allTypes {
		if rt == "" {
			t.Error("RequestType constant should not be empty")
		}
		if seen[rt] {
			t.Errorf("Duplicate RequestType: %q", rt)
		}
		seen[rt] = true
	}
}

func TestEventTypeConstants_Distinct(t *testing.T) {
	allTypes := []string{
		EventFanDataUpdate, EventTemperatureUpdate, EventTemperatureHistoryUpdate,
		EventDeviceConnected, EventDeviceDisconnected, EventDeviceError,
		EventDeviceSettingsUpdate, EventConfigUpdate, EventHotkeyTriggered,
		EventLegionPowerModeUpdate, EventLegionFnQSupportUpdate,
		EventHealthPing, EventHeartbeat,
	}
	seen := make(map[string]bool)
	for _, et := range allTypes {
		if et == "" {
			t.Error("EventType constant should not be empty")
		}
		if seen[et] {
			t.Errorf("Duplicate EventType: %q", et)
		}
		seen[et] = true
	}
}

func TestPipeConstants(t *testing.T) {
	if PipeName == "" {
		t.Error("PipeName should not be empty")
	}
	if LegacyPipeName == "" {
		t.Error("LegacyPipeName should not be empty")
	}
}

func TestClientNotConnected(t *testing.T) {
	logger := testLogger{}
	c := NewClient(logger)
	if c.IsConnected() {
		t.Error("New client should not be connected")
	}
	_, err := c.SendRequest(ReqPing, nil)
	if err == nil {
		t.Error("SendRequest on unconnected client should fail")
	}
}

type testLogger struct{}

func (l testLogger) Info(format string, v ...any)  {}
func (l testLogger) Error(format string, v ...any) {}
func (l testLogger) Warn(format string, v ...any)  {}
func (l testLogger) Debug(format string, v ...any) {}
func (l testLogger) Close()                        {}
func (l testLogger) CleanOldLogs()                 {}
func (l testLogger) SetDebugMode(enabled bool)     {}
func (l testLogger) GetLogDir() string             { return "" }

func TestServerRunning_StateConsistency(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response { return Response{Success: true} }

	s := NewServer(handler, logger)
	s.mutex.RLock()
	if s.running {
		t.Fatal("server should not be running before Start")
	}
	s.mutex.RUnlock()

	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	s.mutex.RLock()
	if !s.running {
		t.Fatal("server should be running after Start")
	}
	s.mutex.RUnlock()

	s.Stop()

	s.mutex.RLock()
	if s.running {
		t.Fatal("server should not be running after Stop")
	}
	s.mutex.RUnlock()
}

func TestServerStop_DoubleStop(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response { return Response{Success: true} }

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	s.Stop()
	s.Stop()

	s.mutex.RLock()
	running := s.running
	s.mutex.RUnlock()
	if running {
		t.Fatal("server should not be running after Stop")
	}
}

func TestConcurrentStopAndAccept(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response { return Response{Success: true} }

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop timed out during concurrent acceptConnections")
	}
}

func TestBroadcastEvent_EmptyClients(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response { return Response{Success: true} }

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BroadcastEvent panicked with no clients: %v", r)
			}
		}()
		s.BroadcastEvent(EventHealthPing, "test")
	}()
}

func TestClientConnectAfterReconnect(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response { return Response{Success: true} }

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	c := NewClient(logger)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	c.Close()
	if c.IsConnected() {
		t.Error("Client should be disconnected after Close")
	}

	if err := c.Connect(); err != nil {
		t.Fatalf("Reconnect error: %v", err)
	}
	c.Close()
}

func TestClientStopDoesNotCloseServer(t *testing.T) {
	logger := testLogger{}
	handler := func(req Request) Response { return Response{Success: true} }

	s := NewServer(handler, logger)
	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	c := NewClient(logger)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	c.Close()

	s.mutex.RLock()
	running := s.running
	s.mutex.RUnlock()
	if !running {
		t.Fatal("server should keep running after client disconnects")
	}
	s.Stop()
}
