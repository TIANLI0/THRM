package plugins

import (
	"context"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (l testLogger) Info(format string, v ...any)    {}
func (l testLogger) Error(format string, v ...any)   {}
func (l testLogger) Warn(format string, v ...any)    {}
func (l testLogger) Debug(format string, v ...any)   {}
func (l testLogger) Close()                          {}
func (l testLogger) CleanOldLogs()                   {}
func (l testLogger) SetDebugMode(enabled bool)       {}
func (l testLogger) GetLogDir() string               { return "" }

type mockPlugin struct {
	id          string
	name        string
	running     bool
	startErr    error
	startCalled bool
	stopCalled  bool
}

func (p *mockPlugin) ID() string          { return p.id }
func (p *mockPlugin) Name() string        { return p.name }
func (p *mockPlugin) Start(ctx context.Context) error {
	p.startCalled = true
	p.running = true
	return p.startErr
}
func (p *mockPlugin) Stop() error {
	p.stopCalled = true
	p.running = false
	return nil
}
func (p *mockPlugin) Status() Status {
	return Status{ID: p.id, Name: p.name, Running: p.running}
}

func TestNewManager(t *testing.T) {
	m := NewManager(testLogger{})
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestRegister(t *testing.T) {
	m := NewManager(testLogger{})
	p := &mockPlugin{id: "test", name: "Test Plugin"}
	m.Register(p)
	statuses := m.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("Statuses len = %d, want 1", len(statuses))
	}
	if statuses[0].ID != "test" {
		t.Errorf("ID = %q, want 'test'", statuses[0].ID)
	}
	if statuses[0].Running {
		t.Error("Plugin should not be running after Register")
	}
}

func TestStartAll(t *testing.T) {
	m := NewManager(testLogger{})
	p1 := &mockPlugin{id: "p1", name: "Plugin 1"}
	p2 := &mockPlugin{id: "p2", name: "Plugin 2"}
	m.Register(p1)
	m.Register(p2)

	if err := m.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll error: %v", err)
	}
	if !p1.startCalled || !p2.startCalled {
		t.Error("All plugins should be started")
	}

	statuses := m.Statuses()
	for _, s := range statuses {
		if !s.Running {
			t.Errorf("Plugin %q should be running after StartAll", s.ID)
		}
	}
}

func TestStartStop_SinglePlugin(t *testing.T) {
	m := NewManager(testLogger{})
	p := &mockPlugin{id: "test", name: "Test"}
	m.Register(p)

	if err := m.Start("test"); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if !p.running {
		t.Error("Plugin should be running after Start")
	}

	if err := m.Stop("test"); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if p.running {
		t.Error("Plugin should not be running after Stop")
	}
}

func TestStart_NotFound(t *testing.T) {
	m := NewManager(testLogger{})
	err := m.Start("nonexistent")
	if err == nil {
		t.Error("Start should fail for non-existent plugin")
	}
}

func TestStopAll(t *testing.T) {
	m := NewManager(testLogger{})
	p1 := &mockPlugin{id: "p1", name: "P1"}
	p2 := &mockPlugin{id: "p2", name: "P2"}
	m.Register(p1)
	m.Register(p2)
	m.StartAll(context.Background())

	m.StopAll()
	if p1.running || p2.running {
		t.Error("All plugins should be stopped after StopAll")
	}
}

func TestStatuses_Empty(t *testing.T) {
	m := NewManager(testLogger{})
	statuses := m.Statuses()
	if len(statuses) != 0 {
		t.Errorf("Statuses on empty manager = %d, want 0", len(statuses))
	}
}

func TestPluginInterface(t *testing.T) {
	p := &mockPlugin{id: "iface", name: "Interface Test"}
	var pi Plugin = p
	if pi.ID() != "iface" {
		t.Errorf("Plugin.ID() = %q", pi.ID())
	}
	if pi.Name() != "Interface Test" {
		t.Errorf("Plugin.Name() = %q", pi.Name())
	}
}

var _ types.Logger = testLogger{}
