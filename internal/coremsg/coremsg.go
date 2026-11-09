// Package coremsg 是核心进程（托盘、系统通知、快捷键提示）面向用户文案的消息目录。
//
// 核心进程常驻后台、开机即运行，往往早于 GUI 启动，因此不能依赖前端的 i18n。
// 这里维护与前端 locales 同名的各语言；zh-CN 的文案与本包引入之前的硬编码完全一致，
// 中文用户看到的任何字符都不应因此改变。日志不经过本包，始终保持中文。
//
// 当前语言是进程级状态：GUI 通过 SetUILanguage 上报后由核心写入配置并调用 SetLanguage；
// 配置里没有记录时按操作系统界面语言推断（见 DetectSystemLanguage）。
package coremsg

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// 支持的界面语言，与前端 SUPPORTED_LOCALES 保持一致。
const (
	ZhCN = "zh-CN"
	EnUS = "en-US"
	JaJP = "ja-JP"
	KoKR = "ko-KR"

	// Default 是无法识别语言时的回退：维持引入多语言之前的行为。
	Default = ZhCN
)

// Supported 返回所有支持的语言标签。
func Supported() []string {
	return []string{ZhCN, EnUS, JaJP, KoKR}
}

// Normalize 把任意语言标签规整为受支持的标签，规则与前端 normalizeLocale 相同：
// 精确匹配优先，其次按 zh/ja/ko/en 前缀归类（zh-TW、en_GB.UTF-8 之类也能落到对应语言）。
// 无法识别时返回 ("", false)，由调用方决定回退策略。
func Normalize(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	for _, lang := range Supported() {
		if value == lang {
			return lang, true
		}
	}

	lowered := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lowered, "zh"):
		return ZhCN, true
	case strings.HasPrefix(lowered, "ja"):
		return JaJP, true
	case strings.HasPrefix(lowered, "ko"):
		return KoKR, true
	case strings.HasPrefix(lowered, "en"):
		return EnUS, true
	}
	return "", false
}

var current atomic.Value // string

// Language 返回当前生效的界面语言。
func Language() string {
	if lang, ok := current.Load().(string); ok && lang != "" {
		return lang
	}
	return Default
}

// SetLanguage 切换当前语言。无法识别的标签被忽略。返回生效的语言以及是否发生了变化。
func SetLanguage(lang string) (string, bool) {
	normalized, ok := Normalize(lang)
	if !ok {
		return Language(), false
	}
	previous := Language()
	current.Store(normalized)
	return normalized, previous != normalized
}

// Resolve 决定配置值对应的实际语言：配置里有可识别的值就用它，
// 否则（新安装、GUI 尚未上报过）按操作系统界面语言推断。
func Resolve(configured string) string {
	if lang, ok := Normalize(configured); ok {
		return lang
	}
	return DetectSystemLanguage()
}

// DetectSystemLanguage 按操作系统界面语言推断，与前端首次启动时按 navigator.language
// 推断的结果保持一致：只看首选语言，不认识的语言回退到 Default。
func DetectSystemLanguage() string {
	if lang, ok := Normalize(systemLanguageTag()); ok {
		return lang
	}
	return Default
}

// T 返回当前语言下的文案。
func T(key Key) string {
	return Lookup(Language(), key)
}

// Tf 返回当前语言下按 fmt 格式化后的文案。各语言的格式动词由测试保证一致。
func Tf(key Key, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// Lookup 返回指定语言下的文案；缺失时回退到 Default，再缺失则返回键名本身，
// 保证界面上至少有东西可看而不是空白。
func Lookup(lang string, key Key) string {
	if table, ok := catalog[lang]; ok {
		if text, ok := table[key]; ok {
			return text
		}
	}
	if text, ok := catalog[Default][key]; ok {
		return text
	}
	return string(key)
}

// gearKeys 把配置/协议里使用的中文挡位标识映射到文案键。挡位标识本身是协议值，
// 不能翻译，只在展示时转换。
var gearKeys = map[string]Key{
	"静音": GearQuiet,
	"标准": GearStandard,
	"强劲": GearStrong,
	"超频": GearOverclock,
}

var levelKeys = map[string]Key{
	"低": LevelLow,
	"中": LevelMedium,
	"高": LevelHigh,
}

// GearName 返回挡位标识（静音/标准/强劲/超频）在当前语言下的显示名；未知标识原样返回。
func GearName(gear string) string {
	if key, ok := gearKeys[gear]; ok {
		return T(key)
	}
	return gear
}

// LevelName 返回小挡位标识（低/中/高）在当前语言下的显示名；未知标识原样返回。
func LevelName(level string) string {
	if key, ok := levelKeys[level]; ok {
		return T(key)
	}
	return level
}
