package coremsg

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// formatVerb 匹配 fmt 格式动词（含 flag/宽度/精度），%% 也算一个以便比较转义。
var formatVerb = regexp.MustCompile(`%[-+# 0]*(?:\d+|\*)?(?:\.(?:\d+|\*))?[a-zA-Z%]`)

func verbsOf(s string) []string {
	return formatVerb.FindAllString(s, -1)
}

// 每个键在三种语言里都必须存在且非空，否则对应语言的用户会看到中文回退或键名。
func TestCatalogCompleteness(t *testing.T) {
	base := catalog[Default]
	if len(base) == 0 {
		t.Fatal("默认语言目录为空")
	}
	for _, lang := range Supported() {
		table, ok := catalog[lang]
		if !ok {
			t.Fatalf("缺少语言 %s", lang)
		}
		for key := range base {
			if strings.TrimSpace(table[key]) == "" {
				t.Errorf("%s 缺少 %q", lang, key)
			}
		}
		for key := range table {
			if _, ok := base[key]; !ok {
				t.Errorf("%s 含有默认语言没有的键 %q", lang, key)
			}
		}
	}
	if len(catalog) != len(Supported()) {
		t.Errorf("目录语言数 %d 与 Supported() %d 不一致", len(catalog), len(Supported()))
	}
}

// 格式动词必须逐个一致（顺序与类型），否则 Tf 在某种语言下会输出 %!d(string=…) 之类的乱码。
func TestCatalogFormatVerbsMatch(t *testing.T) {
	for key, zh := range catalog[Default] {
		want := verbsOf(zh)
		for _, lang := range Supported() {
			got := verbsOf(catalog[lang][key])
			if !slices.Equal(got, want) {
				t.Errorf("%s %q 的格式动词 %v 与 %s 的 %v 不一致", lang, key, got, Default, want)
			}
		}
	}
}

// 中文用户看到的文案必须与引入多语言之前逐字一致。
func TestChineseStringsUnchanged(t *testing.T) {
	cases := map[Key]string{
		TrayTooltipRunning:           "%s - 运行中",
		TrayTooltipSmartControl:      "%s - 智能变频中",
		TrayMenuShow:                 "显示主窗口",
		TrayMenuDeviceConnected:      "设备状态: 已连接",
		TrayMenuCPUTempValue:         "CPU温度: %d°C",
		TrayMenuCPUPowerValue:        "CPU功耗: %.1f W",
		TrayMenuFanSpeedValue:        "风扇转速: %d RPM",
		TrayMenuCurveEmpty:           "暂无可用曲线",
		TrayMenuQuit:                 "退出",
		NotifyDefaultTitle:           "功能变动",
		HotkeyTitle:                  "%s 快捷键",
		HotkeyManualGearRPM:          "手动挡位: %s %s (%d RPM)",
		HotkeyActionFailed:           "%s",
		HotkeyNoCurveProfiles:        "暂无可用温控曲线方案",
		HotkeyAutoBlockedByCustomRPM: "自定义转速模式下无法开启智能变频",
	}
	for key, want := range cases {
		if got := Lookup(ZhCN, key); got != want {
			t.Errorf("%q = %q, want %q", key, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"zh-CN", ZhCN, true},
		{"en-US", EnUS, true},
		{"ja-JP", JaJP, true},
		{" en-GB ", EnUS, true},
		{"zh-TW", ZhCN, true},
		{"ZH-hans", ZhCN, true},
		{"ja", JaJP, true},
		{"ko-KR", KoKR, true},
		{"ko", KoKR, true},
		{"de-DE", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := Normalize(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Normalize(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestSetLanguageAndLookup(t *testing.T) {
	t.Cleanup(func() { SetLanguage(Default) })

	if got := Language(); got != Default {
		t.Fatalf("初始语言应为 %s，得到 %s", Default, got)
	}
	if lang, changed := SetLanguage("en-US"); lang != EnUS || !changed {
		t.Fatalf("SetLanguage(en-US) = (%q, %v)", lang, changed)
	}
	if _, changed := SetLanguage("en-US"); changed {
		t.Fatal("重复设置同一语言不应报告变化")
	}
	if lang, changed := SetLanguage("fr-FR"); lang != EnUS || changed {
		t.Fatalf("无法识别的语言应被忽略，得到 (%q, %v)", lang, changed)
	}
	if got := Tf(TrayMenuFanSpeedValue, 1800); got != "Fan speed: 1800 RPM" {
		t.Errorf("Tf = %q", got)
	}
	if got := GearName("强劲"); got != "Strong" {
		t.Errorf("GearName = %q", got)
	}
	if got := LevelName("高"); got != "High" {
		t.Errorf("LevelName = %q", got)
	}
	if got := GearName("自定义"); got != "自定义" {
		t.Errorf("未知挡位应原样返回，得到 %q", got)
	}

	SetLanguage(JaJP)
	if got := T(TrayMenuQuit); got != "終了" {
		t.Errorf("ja T = %q", got)
	}

	SetLanguage(KoKR)
	if got := T(TrayMenuQuit); got != "종료" {
		t.Errorf("ko T = %q", got)
	}
	if got := GearName("静音"); got != "저소음" {
		t.Errorf("ko GearName = %q", got)
	}
}

func TestLookupFallsBack(t *testing.T) {
	if got := Lookup("xx-XX", TrayMenuQuit); got != "退出" {
		t.Errorf("未知语言应回退到默认语言，得到 %q", got)
	}
	if got := Lookup(EnUS, Key("no.such.key")); got != "no.such.key" {
		t.Errorf("未知键应返回键名，得到 %q", got)
	}
}

func TestResolvePrefersConfiguredLanguage(t *testing.T) {
	if got := Resolve("ja-JP"); got != JaJP {
		t.Errorf("Resolve(ja-JP) = %q", got)
	}
	// 未配置时按系统语言推断，结果一定是受支持的语言之一。
	if got := Resolve(""); !slices.Contains(Supported(), got) {
		t.Errorf("Resolve(\"\") = %q 不是受支持的语言", got)
	}
}

func TestLanguageFromLocaleEnv(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	cases := []struct {
		values map[string]string
		want   string
	}{
		{map[string]string{"LANG": "zh_CN.UTF-8"}, "zh-CN"},
		{map[string]string{"LANG": "en_US.UTF-8", "LC_MESSAGES": "ja_JP.UTF-8"}, "ja-JP"},
		{map[string]string{"LANG": "ja_JP.UTF-8", "LC_ALL": "en_GB.UTF-8"}, "en-GB"},
		{map[string]string{"LANG": "ko_KR.UTF-8"}, "ko-KR"},
		{map[string]string{"LANG": "de_DE@euro"}, "de-DE"},
		{map[string]string{"LC_ALL": "C", "LANG": "zh_CN.UTF-8"}, ""},
		{map[string]string{"LANG": "C.UTF-8"}, ""},
		{map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := languageFromLocaleEnv(env(c.values)); got != c.want {
			t.Errorf("languageFromLocaleEnv(%v) = %q, want %q", c.values, got, c.want)
		}
	}
}
