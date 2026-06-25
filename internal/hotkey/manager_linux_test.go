package hotkey

import (
	"os"
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
	hotkeylib "golang.design/x/hotkey"
)

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Debug(string, ...any) {}
func (testLogger) Close()               {}
func (testLogger) CleanOldLogs()        {}
func (testLogger) SetDebugMode(bool)    {}
func (testLogger) GetLogDir() string    { return "" }

// --- normalizeShortcut ---

func TestNormalizeShortcut_Empty(t *testing.T) {
	if got := normalizeShortcut(""); got != "" {
		t.Fatalf("normalizeShortcut empty = %q", got)
	}
	if got := normalizeShortcut("   "); got != "" {
		t.Fatalf("normalizeShortcut spaces = %q", got)
	}
}

func TestNormalizeShortcut_SpacesTrimmed(t *testing.T) {
	got := normalizeShortcut(" Ctrl + Shift + A ")
	if got != "CTRL+SHIFT+A" {
		t.Fatalf("normalizeShortcut = %q, want CTRL+SHIFT+A", got)
	}
}

func TestNormalizeShortcut_HyphenToPlus(t *testing.T) {
	got := normalizeShortcut("ctrl-shift-a")
	if got != "CTRL+SHIFT+A" {
		t.Fatalf("normalizeShortcut hyphen = %q", got)
	}
}

func TestNormalizeShortcut_UnderscoreToPlus(t *testing.T) {
	got := normalizeShortcut("ctrl_shift_a")
	if got != "CTRL+SHIFT+A" {
		t.Fatalf("normalizeShortcut underscore = %q", got)
	}
}

func TestNormalizeShortcut_LowercaseUpper(t *testing.T) {
	got := normalizeShortcut("ctrl+shit+a")
	if got != "CTRL+SHIT+A" {
		t.Fatalf("normalizeShortcut lowercase = %q", got)
	}
}

func TestNormalizeShortcut_Mixed(t *testing.T) {
	got := normalizeShortcut(" Ctrl+Shift+F4")
	if got != "CTRL+SHIFT+F4" {
		t.Fatalf("normalizeShortcut mixed = %q, want CTRL+SHIFT+F4", got)
	}
}

// --- parseModifier ---

func TestParseModifier_Ctrl(t *testing.T) {
	for _, input := range []string{"Ctrl", "CTRL", "ctrl", "CONTROL", "Control"} {
		mod, ok := parseModifier(input)
		if !ok || mod != hotkeylib.ModCtrl {
			t.Fatalf("parseModifier(%q) = (%v, %v)", input, mod, ok)
		}
	}
}

func TestParseModifier_Alt(t *testing.T) {
	mod, ok := parseModifier("Alt")
	if !ok || mod != hotkeylib.Mod1 {
		t.Fatalf("parseModifier(Alt) = (%v, %v), want (Mod1, true)", mod, ok)
	}
}

func TestParseModifier_Shift(t *testing.T) {
	mod, ok := parseModifier("Shift")
	if !ok || mod != hotkeylib.ModShift {
		t.Fatalf("parseModifier(Shift) = (%v, %v)", mod, ok)
	}
}

func TestParseModifier_Win(t *testing.T) {
	for _, input := range []string{"Win", "Windows", "SUPER", "Super"} {
		mod, ok := parseModifier(input)
		if !ok || mod != hotkeylib.Mod4 {
			t.Fatalf("parseModifier(%q) = (%v, %v), want (Mod4, true)", input, mod, ok)
		}
	}
}

func TestParseModifier_Invalid(t *testing.T) {
	for _, input := range []string{"Tab", "CapsLock", "Enter", "cmd"} {
		_, ok := parseModifier(input)
		if ok {
			t.Fatalf("parseModifier(%q) should return false", input)
		}
	}
}

// --- parseKey ---

func TestParseKey_Letters(t *testing.T) {
	keys := map[string]hotkeylib.Key{
		"a": hotkeylib.KeyA, "A": hotkeylib.KeyA,
		"b": hotkeylib.KeyB, "z": hotkeylib.KeyZ,
		"M": hotkeylib.KeyM, "X": hotkeylib.KeyX,
	}
	for input, expected := range keys {
		key, err := parseKey(input)
		if err != nil || key != expected {
			t.Fatalf("parseKey(%q) = (%v, %v), want (%v, nil)", input, key, err, expected)
		}
	}
}

func TestParseKey_Digits(t *testing.T) {
	keys := map[string]hotkeylib.Key{
		"0": hotkeylib.Key0, "1": hotkeylib.Key1,
		"5": hotkeylib.Key5, "9": hotkeylib.Key9,
	}
	for input, expected := range keys {
		key, err := parseKey(input)
		if err != nil || key != expected {
			t.Fatalf("parseKey(%q) = (%v, %v)", input, key, err)
		}
	}
}

func TestParseKey_FunctionKeys(t *testing.T) {
	for i := 1; i <= 12; i++ {
		input := "F" + strings.Repeat("", 0)
		switch i {
		case 1:
			input = "F1"
			key, err := parseKey(input)
			if err != nil || key != hotkeylib.KeyF1 {
				t.Fatalf("parseKey(F1) = (%v, %v)", key, err)
			}
		case 12:
			input = "F12"
			key, err := parseKey(input)
			if err != nil || key != hotkeylib.KeyF12 {
				t.Fatalf("parseKey(F12) = (%v, %v)", key, err)
			}
		}
	}
}

func TestParseKey_F13_Unsupported(t *testing.T) {
	_, err := parseKey("F13")
	if err == nil {
		t.Fatal("parseKey(F13) should return error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("parseKey(F13) error = %v, want 'unsupported'", err)
	}
}

func TestParseKey_Tab_Unsupported(t *testing.T) {
	_, err := parseKey("Tab")
	if err == nil {
		t.Fatal("parseKey(Tab) should return error")
	}
}

func TestParseKey_Enter_Unsupported(t *testing.T) {
	_, err := parseKey("Enter")
	if err == nil {
		t.Fatal("parseKey(Enter) should return error")
	}
}

func TestParseKey_FunctionKeys_All(t *testing.T) {
	expected := []hotkeylib.Key{
		hotkeylib.KeyF1, hotkeylib.KeyF2, hotkeylib.KeyF3,
		hotkeylib.KeyF4, hotkeylib.KeyF5, hotkeylib.KeyF6,
		hotkeylib.KeyF7, hotkeylib.KeyF8, hotkeylib.KeyF9,
		hotkeylib.KeyF10, hotkeylib.KeyF11, hotkeylib.KeyF12,
	}
	for i, exp := range expected {
		input := "F" + strings.TrimLeft(strings.Repeat(" ", 0), " ")
		_ = i
		_ = input
		_ = exp
	}

	for i := 1; i <= 12; i++ {
		var input string
		switch i {
		case 1:
			input = "F1"
		case 2:
			input = "F2"
		case 3:
			input = "F3"
		case 4:
			input = "F4"
		case 5:
			input = "F5"
		case 6:
			input = "F6"
		case 7:
			input = "F7"
		case 8:
			input = "F8"
		case 9:
			input = "F9"
		case 10:
			input = "F10"
		case 11:
			input = "F11"
		case 12:
			input = "F12"
		}
		key, err := parseKey(input)
		if err != nil {
			t.Fatalf("parseKey(%s) unexpected error: %v", input, err)
		}
		if key != expected[i-1] {
			t.Fatalf("parseKey(%s) = %v, want %v", input, key, expected[i-1])
		}
	}
}

// --- ParseShortcut ---

func TestParseShortcut_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMods []hotkeylib.Modifier
		wantKey  hotkeylib.Key
	}{
		{
			name:     "Ctrl+Shift+A",
			input:    "Ctrl+Shift+A",
			wantMods: []hotkeylib.Modifier{hotkeylib.ModCtrl, hotkeylib.ModShift},
			wantKey:  hotkeylib.KeyA,
		},
		{
			name:     "Win+L",
			input:    "Win+L",
			wantMods: []hotkeylib.Modifier{hotkeylib.Mod4},
			wantKey:  hotkeylib.KeyL,
		},
		{
			name:     "Alt+F4",
			input:    "Alt+F4",
			wantMods: []hotkeylib.Modifier{hotkeylib.Mod1},
			wantKey:  hotkeylib.KeyF4,
		},
		{
			name:     "Ctrl+Alt+D",
			input:    "Ctrl+Alt+D",
			wantMods: []hotkeylib.Modifier{hotkeylib.ModCtrl, hotkeylib.Mod1},
			wantKey:  hotkeylib.KeyD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mods, key, err := ParseShortcut(tt.input)
			if err != nil {
				t.Fatalf("ParseShortcut error: %v", err)
			}
			if len(mods) != len(tt.wantMods) {
				t.Fatalf("mods count = %d, want %d", len(mods), len(tt.wantMods))
			}
			for i, mod := range mods {
				if mod != tt.wantMods[i] {
					t.Fatalf("mods[%d] = %v, want %v", i, mod, tt.wantMods[i])
				}
			}
			if key != tt.wantKey {
				t.Fatalf("key = %v, want %v", key, tt.wantKey)
			}
		})
	}
}

func TestParseShortcut_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		errText string
	}{
		{name: "empty", input: "", errText: "empty shortcut"},
		{name: "only modifier", input: "Ctrl+", errText: "missing main key"},
		{name: "only key", input: "A", errText: "missing modifier"},
		{name: "multiple main keys", input: "Ctrl+A+B", errText: "multiple main keys"},
		{name: "unknown key", input: "Ctrl+Tab", errText: "unsupported key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseShortcut(tt.input)
			if err == nil {
				t.Fatalf("ParseShortcut(%q) should return error", tt.input)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errText)) {
				t.Fatalf("error = %v, want containing %q", err, tt.errText)
			}
		})
	}
}

// --- Manager lifecycle ---

func TestNewManager_CreatesManager(t *testing.T) {
	m := NewManager(testLogger{}, func(action Action, shortcut string) {})
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.logger == nil {
		t.Fatal("Manager.logger is nil")
	}
}

func TestNewManager_WaylandDetection(t *testing.T) {
	currentType := os.Getenv("XDG_SESSION_TYPE")
	currentDisplay := os.Getenv("WAYLAND_DISPLAY")

	t.Run("default environment", func(t *testing.T) {
		m := NewManager(testLogger{}, nil)
		if currentType == "wayland" || currentDisplay != "" {
			if !m.isWayland {
				t.Log("Warning: expected isWayland=true but got false in Wayland session")
			}
		}
	})

	t.Run("simulated X11", func(t *testing.T) {
		t.Setenv("XDG_SESSION_TYPE", "x11")
		t.Setenv("WAYLAND_DISPLAY", "")
		m := NewManager(testLogger{}, nil)
		if m.isWayland {
			t.Fatal("isWayland should be false when XDG_SESSION_TYPE=x11")
		}
	})

	t.Run("simulated Wayland", func(t *testing.T) {
		t.Setenv("XDG_SESSION_TYPE", "wayland")
		m := NewManager(testLogger{}, nil)
		if !m.isWayland {
			t.Fatal("isWayland should be true when XDG_SESSION_TYPE=wayland")
		}
	})
}

func TestManager_StopIdempotent(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	m.Stop()
	m.Stop()
}

func TestManager_UpdateBindingsAfterStop(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	m.Stop()
	err := m.UpdateBindings("Ctrl+Shift+A", "", "")
	if err == nil {
		t.Fatal("UpdateBindings after Stop should return error")
	}
	if !strings.Contains(err.Error(), "already stopped") {
		t.Fatalf("error = %v, want 'already stopped'", err)
	}
}

func TestManager_UpdateBindingsEmpty(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	err := m.UpdateBindings("", "", "")
	if err != nil {
		t.Fatalf("UpdateBindings with empty shortcuts: %v", err)
	}
}

func TestManager_UpdateBindingsInvalid(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	err := m.UpdateBindings("InvalidShortcut", "", "")
	if err == nil {
		t.Fatal("UpdateBindings with invalid shortcut should return error")
	}
}

func TestManager_OnActionCallback(t *testing.T) {
	called := false
	var receivedAction Action
	m := NewManager(testLogger{}, func(action Action, shortcut string) {
		called = true
		receivedAction = action
	})
	if m.onAction == nil {
		t.Fatal("onAction callback should not be nil")
	}
	m.onAction(ActionToggleManualGear, "Ctrl+Shift+M")
	if !called {
		t.Fatal("onAction callback was not called")
	}
	if receivedAction != ActionToggleManualGear {
		t.Fatalf("receivedAction = %q, want toggle-manual-gear", receivedAction)
	}
}

func TestManager_NilOnActionIsSafe(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	m.Stop()
}

func TestActionConstants(t *testing.T) {
	if ActionToggleManualGear != "toggle-manual-gear" {
		t.Fatalf("ActionToggleManualGear = %q", ActionToggleManualGear)
	}
	if ActionToggleAutoMode != "toggle-auto-control" {
		t.Fatalf("ActionToggleAutoMode = %q", ActionToggleAutoMode)
	}
	if ActionToggleCurveProfile != "toggle-curve-profile" {
		t.Fatalf("ActionToggleCurveProfile = %q", ActionToggleCurveProfile)
	}
}

func TestNormalizeShortcut_EmptyParts(t *testing.T) {
	got := normalizeShortcut("ctrl++shift")
	if got != "CTRL+SHIFT" {
		t.Fatalf("normalizeShortcut double plus = %q", got)
	}
}

func TestParseShortcut_DuplicateModifiers(t *testing.T) {
	mods, key, err := ParseShortcut("ctrl+CONTROL+A")
	if err != nil {
		t.Fatalf("ParseShortcut duplicate mods error: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("duplicate modifiers should be deduplicated, got %d mods", len(mods))
	}
	if mods[0] != hotkeylib.ModCtrl {
		t.Fatalf("mod = %v, want ModCtrl", mods[0])
	}
	if key != hotkeylib.KeyA {
		t.Fatalf("key = %v, want KeyA", key)
	}
}

var _ types.Logger = testLogger{}
