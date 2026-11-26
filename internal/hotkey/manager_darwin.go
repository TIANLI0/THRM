//go:build darwin

// macOS 全局快捷键实现。
//
// 用 Carbon 的 RegisterEventHotKey 注册系统级热键，事件由
// GetApplicationEventTarget() 派发到 handler——也就是处理托盘消息循环的那个
// RunLoop（main goroutine 上 [NSApp run] 提供的）。所以必须在托盘启动之后
// 才注册，否则 InstallEventHandler 挂上去也没有循环去跑它。
//
// 不需要辅助功能权限：RegisterEventHotKey 是系统注册，只要求进程有
// 事件循环（我们的托盘恰好提供了）。

package hotkey

/*
#cgo LDFLAGS: -framework Carbon
#include <Carbon/Carbon.h>

#define THRM_HOTKEY_MAX 8

static EventHotKeyRef gHotkeyRefs[THRM_HOTKEY_MAX];
static int gHotkeyRegistered[THRM_HOTKEY_MAX];
static EventHandlerRef gHotkeyHandler = NULL;
static void (*gHotkeyCallback)(int) = NULL;

// 由 hotkey_export_darwin.go 里 //export 生成，cgo 会把它暴露成 C 符号。
extern void thrmGoHotkeyFired(int id);

static OSStatus thrmHotkeyHandler(EventHandlerCallRef nextHandler, EventRef event, void *userData) {
    EventHotKeyID hkID;
    OSStatus err = GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID,
                                     NULL, sizeof(hkID), NULL, &hkID);
    if (err == noErr && gHotkeyCallback) {
        gHotkeyCallback((int)hkID.id);
    }
    return noErr;
}

static int thrmHotkeyInstall(void) {
    gHotkeyCallback = thrmGoHotkeyFired;
    if (gHotkeyHandler) return 0;
    EventTypeSpec spec = { kEventClassKeyboard, kEventHotKeyPressed };
    OSStatus err = InstallEventHandler(GetApplicationEventTarget(),
                                       NewEventHandlerUPP(thrmHotkeyHandler),
                                       1, &spec, NULL, &gHotkeyHandler);
    return (err == noErr) ? 0 : (int)err;
}

static int thrmHotkeyRegister(int id, int keyCode, int modifiers) {
    if (id < 0 || id >= THRM_HOTKEY_MAX) return -1;
    if (gHotkeyRegistered[id]) {
        UnregisterEventHotKey(gHotkeyRefs[id]);
        gHotkeyRefs[id] = 0;
        gHotkeyRegistered[id] = 0;
    }
    EventHotKeyID hkID;
    hkID.signature = 0x5448524D; // 'THRM'
    hkID.id = (UInt32)id;
    OSStatus err = RegisterEventHotKey((UInt32)keyCode, (UInt32)modifiers, hkID,
                                       GetApplicationEventTarget(), 0, &gHotkeyRefs[id]);
    if (err != noErr) return (int)err;
    gHotkeyRegistered[id] = 1;
    return 0;
}

static void thrmHotkeyUnregister(int id) {
    if (id < 0 || id >= THRM_HOTKEY_MAX || !gHotkeyRegistered[id]) return;
    UnregisterEventHotKey(gHotkeyRefs[id]);
    gHotkeyRefs[id] = 0;
    gHotkeyRegistered[id] = 0;
}

// Carbon 修饰键位
static int thrmModControl(void)   { return controlKey; }
static int thrmModOption(void)    { return optionKey; }
static int thrmModShift(void)     { return shiftKey; }
static int thrmModCommand(void)   { return cmdKey; }
*/
import "C"

import (
	"fmt"
	"strings"
	"sync"

	"github.com/TIANLI0/THRM/internal/types"
)

// Action 定义快捷键触发的动作。
type Action string

const (
	ActionToggleManualGear   Action = "toggle-manual-gear"
	ActionToggleAutoMode     Action = "toggle-auto-control"
	ActionToggleCurveProfile Action = "toggle-curve-profile"
)

// Modifier 是修饰键位（内部表示，映射到 Carbon 的 controlKey 等）。
type Modifier uint32

// Key 是 macOS 虚拟键码（HIToolbox/Events.h 的 kVK_*）。
type Key uint32

const (
	ModCtrl  Modifier = 1 << 0
	ModAlt   Modifier = 1 << 1
	ModShift Modifier = 1 << 2
	ModCmd   Modifier = 1 << 3
)

// actionOrder 固定 id 分配，和 C 侧的数组下标一致。
var actionOrder = []Action{
	ActionToggleManualGear,
	ActionToggleAutoMode,
	ActionToggleCurveProfile,
}

// macOS 虚拟键码（kVK_ANSI_*）。
var darwinKeyCodes = map[string]Key{
	"A": 0x00, "S": 0x01, "D": 0x02, "F": 0x03, "H": 0x04, "G": 0x05,
	"Z": 0x06, "X": 0x07, "C": 0x08, "V": 0x09, "B": 0x0B, "Q": 0x0C,
	"W": 0x0D, "E": 0x0E, "R": 0x0F, "Y": 0x10, "T": 0x11, "1": 0x12,
	"2": 0x13, "3": 0x14, "4": 0x15, "6": 0x16, "5": 0x17, "9": 0x19,
	"7": 0x1A, "8": 0x1C, "0": 0x1D, "O": 0x1F, "U": 0x20, "I": 0x22,
	"P": 0x23, "L": 0x25, "J": 0x26, "K": 0x28, "N": 0x2D, "M": 0x2E,
	"F1": 0x7A, "F2": 0x78, "F3": 0x63, "F4": 0x76, "F5": 0x60, "F6": 0x61,
	"F7": 0x62, "F8": 0x64, "F9": 0x65, "F10": 0x6D, "F11": 0x67, "F12": 0x6F,
	"LEFT": 0x7B, "RIGHT": 0x7C, "DOWN": 0x7D, "UP": 0x7E,
	"SPACE": 0x31, "RETURN": 0x24, "TAB": 0x30, "ESCAPE": 0x35,
}

// activeManager 是当前生效的管理器，供 C 回调找到实例。
var activeManager *Manager

// Manager 负责注册/更新/注销全局快捷键。
type Manager struct {
	logger   types.Logger
	onAction func(action Action, shortcut string)

	mutex    sync.Mutex
	bindings map[Action]string
	keys     map[Action]Key
	mods     map[Action]Modifier
}

// NewManager 创建一个全局快捷键管理器。
func NewManager(logger types.Logger, onAction func(action Action, shortcut string)) *Manager {
	m := &Manager{
		logger:   logger,
		onAction: onAction,
		bindings: make(map[Action]string),
		keys:     make(map[Action]Key),
		mods:     make(map[Action]Modifier),
	}
	activeManager = m
	// 安装一次事件处理器；重复调用是安全的。
	C.thrmHotkeyInstall()
	return m
}

// UpdateBindings 解析并注册三组快捷键。
func (m *Manager) UpdateBindings(manualGearShortcut, autoControlShortcut, curveProfileShortcut string) error {
	wanted := map[Action]string{
		ActionToggleManualGear:   manualGearShortcut,
		ActionToggleAutoMode:     autoControlShortcut,
		ActionToggleCurveProfile: curveProfileShortcut,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id, action := range actionOrder {
		shortcut := strings.TrimSpace(wanted[action])
		if shortcut == "" {
			C.thrmHotkeyUnregister(C.int(id))
			delete(m.bindings, action)
			delete(m.keys, action)
			delete(m.mods, action)
			continue
		}

		mods, key, err := ParseShortcut(shortcut)
		if err != nil {
			return fmt.Errorf("解析快捷键 %q 失败: %w", shortcut, err)
		}

		carbonMods := C.int(0)
		for _, md := range mods {
			switch md {
			case ModCtrl:
				carbonMods |= C.thrmModControl()
			case ModAlt:
				carbonMods |= C.thrmModOption()
			case ModShift:
				carbonMods |= C.thrmModShift()
			case ModCmd:
				carbonMods |= C.thrmModCommand()
			}
		}

		if rc := C.thrmHotkeyRegister(C.int(id), C.int(key), carbonMods); rc != 0 {
			return fmt.Errorf("注册快捷键 %q 失败（可能被其他程序占用），Carbon 错误码 %d", shortcut, int(rc))
		}

		m.bindings[action] = shortcut
		m.keys[action] = key
		m.mods[action] = mods[0]
		m.logInfo("已注册全局快捷键 %s → %s", shortcut, action)
	}
	return nil
}

// Stop 注销全部快捷键。
func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id := range actionOrder {
		C.thrmHotkeyUnregister(C.int(id))
	}
	m.bindings = make(map[Action]string)
	m.keys = make(map[Action]Key)
	m.mods = make(map[Action]Modifier)
}

// dispatch 由 C 侧事件处理器回调。
func (m *Manager) dispatch(id int) {
	if id < 0 || id >= len(actionOrder) {
		return
	}
	action := actionOrder[id]

	m.mutex.Lock()
	shortcut, ok := m.bindings[action]
	cb := m.onAction
	m.mutex.Unlock()

	if !ok || cb == nil {
		return
	}
	m.logInfo("快捷键触发: %s (%s)", action, shortcut)
	cb(action, shortcut)
}

// ParseShortcut 解析 "Ctrl+Alt+Shift+M" 形式，返回修饰键与 macOS 键码。
func ParseShortcut(input string) ([]Modifier, Key, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil, 0, fmt.Errorf("快捷键为空")
	}

	parts := strings.Split(s, "+")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("快捷键 %q 需要至少一个修饰键", input)
	}

	var mods []Modifier
	for _, raw := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "ctrl", "control":
			mods = append(mods, ModCtrl)
		case "alt", "option", "opt":
			mods = append(mods, ModAlt)
		case "shift":
			mods = append(mods, ModShift)
		case "cmd", "command", "win", "super":
			mods = append(mods, ModCmd)
		default:
			return nil, 0, fmt.Errorf("未知修饰键 %q", raw)
		}
	}
	if len(mods) == 0 {
		return nil, 0, fmt.Errorf("快捷键 %q 缺少修饰键", input)
	}

	keyName := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	key, ok := darwinKeyCodes[keyName]
	if !ok {
		return nil, 0, fmt.Errorf("不支持的按键 %q", keyName)
	}
	return mods, key, nil
}

func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}
