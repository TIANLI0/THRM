//go:build windows

package autostart

import (
	"os"
	"strings"
	"testing"
	"unicode/utf16"
)

// 任务定义必须显式关掉两个电池相关默认值，并解除运行时长限制：
// 前者会让笔记本拔掉电源开机时压根不启动（或切到电池时被杀掉），
// 后者会在开机满 72 小时后终止常驻核心。
func TestBuildScheduledTaskXMLBatteryAndRuntimeSettings(t *testing.T) {
	definition := buildScheduledTaskXML(`C:\Program Files\THRM\THRM Core.exe`, "S-1-5-21-1-2-3-1001")

	var parsed struct {
		Settings struct {
			DisallowStartIfOnBatteries bool   `xml:"DisallowStartIfOnBatteries"`
			StopIfGoingOnBatteries     bool   `xml:"StopIfGoingOnBatteries"`
			ExecutionTimeLimit         string `xml:"ExecutionTimeLimit"`
			StartWhenAvailable         bool   `xml:"StartWhenAvailable"`
			MultipleInstancesPolicy    string `xml:"MultipleInstancesPolicy"`
		} `xml:"Settings"`
		Triggers struct {
			LogonTrigger struct {
				Enabled bool   `xml:"Enabled"`
				UserID  string `xml:"UserId"`
			} `xml:"LogonTrigger"`
		} `xml:"Triggers"`
		Principals struct {
			Principal struct {
				UserID    string `xml:"UserId"`
				LogonType string `xml:"LogonType"`
				RunLevel  string `xml:"RunLevel"`
			} `xml:"Principal"`
		} `xml:"Principals"`
		Actions struct {
			Exec struct {
				Command   string `xml:"Command"`
				Arguments string `xml:"Arguments"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	if err := parseTaskDefinition(definition, &parsed); err != nil {
		t.Fatalf("生成的任务定义不是合法 XML: %v\n%s", err, definition)
	}

	if parsed.Settings.DisallowStartIfOnBatteries {
		t.Fatal("DisallowStartIfOnBatteries 必须为 false，否则电池供电时不会自启动")
	}
	if parsed.Settings.StopIfGoingOnBatteries {
		t.Fatal("StopIfGoingOnBatteries 必须为 false，否则切到电池会终止核心")
	}
	if parsed.Settings.ExecutionTimeLimit != "PT0S" {
		t.Fatalf("ExecutionTimeLimit = %q，常驻服务必须不限运行时长(PT0S)", parsed.Settings.ExecutionTimeLimit)
	}
	if !parsed.Settings.StartWhenAvailable {
		t.Fatal("StartWhenAvailable 应为 true，错过的触发要能补跑")
	}
	if parsed.Settings.MultipleInstancesPolicy != "IgnoreNew" {
		t.Fatalf("MultipleInstancesPolicy = %q, want IgnoreNew", parsed.Settings.MultipleInstancesPolicy)
	}
	if !parsed.Triggers.LogonTrigger.Enabled {
		t.Fatal("登录触发器必须启用")
	}
	if parsed.Triggers.LogonTrigger.UserID != "S-1-5-21-1-2-3-1001" {
		t.Fatalf("触发器 UserId = %q", parsed.Triggers.LogonTrigger.UserID)
	}
	if parsed.Principals.Principal.LogonType != "InteractiveToken" {
		t.Fatalf("LogonType = %q，需要交互式令牌才能显示托盘", parsed.Principals.Principal.LogonType)
	}
	if parsed.Principals.Principal.RunLevel != "HighestAvailable" {
		t.Fatalf("RunLevel = %q", parsed.Principals.Principal.RunLevel)
	}
	if parsed.Actions.Exec.Command != `C:\Program Files\THRM\THRM Core.exe` {
		t.Fatalf("Command = %q", parsed.Actions.Exec.Command)
	}
	if parsed.Actions.Exec.Arguments != "--autostart" {
		t.Fatalf("Arguments = %q", parsed.Actions.Exec.Arguments)
	}
	if strings.Contains(definition, "<Delay>") {
		t.Fatal("不应再有登录延时：托盘注册自己会等任务栏就绪")
	}
}

// 账户解析不出来时仍要生成合法定义（省略 UserId，由 schtasks 用当前用户注册）。
func TestBuildScheduledTaskXMLWithoutAccount(t *testing.T) {
	definition := buildScheduledTaskXML(`C:\THRM\core.exe`, "")
	if strings.Contains(definition, "<UserId>") {
		t.Fatal("无账户时不应写出空的 UserId")
	}
	var probe struct{}
	if err := parseTaskDefinition(definition, &probe); err != nil {
		t.Fatalf("生成的任务定义不是合法 XML: %v", err)
	}
}

// 路径里的 & 等字符必须转义，否则 schtasks 会拒绝整个定义。
func TestBuildScheduledTaskXMLEscapesCommand(t *testing.T) {
	definition := buildScheduledTaskXML(`C:\Tools & Utils\core.exe`, "")
	if !strings.Contains(definition, "&amp;") {
		t.Fatal("命令路径中的 & 未转义")
	}
	var parsed struct {
		Command string `xml:"Actions>Exec>Command"`
	}
	if err := parseTaskDefinition(definition, &parsed); err != nil {
		t.Fatalf("生成的任务定义不是合法 XML: %v", err)
	}
	if parsed.Command != `C:\Tools & Utils\core.exe` {
		t.Fatalf("Command = %q", parsed.Command)
	}
}

func TestScheduledTaskNeedsUpgrade(t *testing.T) {
	current := buildScheduledTaskXML(`C:\THRM\core.exe`, "S-1-5-21-1-2-3-1001")
	if scheduledTaskNeedsUpgrade(current) {
		t.Fatal("当前定义不应被判为需要升级")
	}

	// schtasks 命令行创建的旧任务：两个电池设置为 true，运行时长限制 72 小时。
	legacy := `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Settings>
    <DisallowStartIfOnBatteries>true</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>true</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT72H</ExecutionTimeLimit>
  </Settings>
</Task>`
	if !scheduledTaskNeedsUpgrade(legacy) {
		t.Fatal("旧定义应被判为需要升级")
	}

	// 只差运行时长一项也要升级。
	partial := strings.ReplaceAll(strings.ReplaceAll(legacy,
		"<DisallowStartIfOnBatteries>true<", "<DisallowStartIfOnBatteries>false<"),
		"<StopIfGoingOnBatteries>true<", "<StopIfGoingOnBatteries>false<")
	if !scheduledTaskNeedsUpgrade(partial) {
		t.Fatal("仅 ExecutionTimeLimit 落后也应升级")
	}

	// 缺失字段等同于 Windows 默认值(true)，也要升级。
	missing := `<Task version="1.2"><Settings><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings></Task>`
	if !scheduledTaskNeedsUpgrade(missing) {
		t.Fatal("缺失电池设置应按默认值判为需要升级")
	}

	// 解析失败时不动既有任务，避免每次启动都重建。
	if scheduledTaskNeedsUpgrade("not xml at all") {
		t.Fatal("无法解析的定义不应触发重建")
	}
}

// schtasks /query /xml 输出的是 UTF-16LE（带 BOM），必须能解出可解析的文本。
func TestDecodeSchtasksOutputUTF16(t *testing.T) {
	source := `<Task><Settings><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings></Task>`
	encoded := []byte{0xFF, 0xFE}
	for _, unit := range utf16.Encode([]rune(source)) {
		encoded = append(encoded, byte(unit), byte(unit>>8))
	}
	if got := decodeSchtasksOutput(encoded); got != source {
		t.Fatalf("decodeSchtasksOutput = %q, want %q", got, source)
	}

	if got := decodeSchtasksOutput([]byte("plain ascii")); got != "plain ascii" {
		t.Fatalf("非 UTF-16 输出应原样返回, got %q", got)
	}
}

// 带 UTF-8 BOM 的定义同样要能解析。
func TestScheduledTaskNeedsUpgradeSkipsBOM(t *testing.T) {
	definition := string([]byte{0xEF, 0xBB, 0xBF}) + buildScheduledTaskXML(`C:\THRM\core.exe`, "")
	if scheduledTaskNeedsUpgrade(definition) {
		t.Fatal("BOM 不应导致解析失败并误判")
	}
}

// 任务定义文件必须是 schtasks 能接受的 UTF-16LE(带 BOM)。
func TestWriteTaskXMLFileEncoding(t *testing.T) {
	path, cleanup, err := writeTaskXMLFile("<Task/>")
	if err != nil {
		t.Fatalf("writeTaskXMLFile error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xFE {
		t.Fatalf("缺少 UTF-16LE BOM: % x", data)
	}
	if got := decodeSchtasksOutput(data); got != "<Task/>" {
		t.Fatalf("回读内容 = %q", got)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cleanup 应删除临时文件")
	}
}
