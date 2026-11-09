package coremsg

// Key 标识一条面向用户的文案。
type Key string

// 托盘悬浮提示。%s 为应用名。
//
// 悬浮提示受 Windows 64 个 UTF-16 单元的硬上限约束（见 tray.trayTooltipMaxUnits），
// 译文要短：英文"智能变频中"用 Smart mode 而不是 Smart control，才能在最坏读数下留住三行。
const (
	TrayTooltipRunning      Key = "tray.tooltip.running"
	TrayTooltipDisconnected Key = "tray.tooltip.disconnected"
	TrayTooltipSmartControl Key = "tray.tooltip.smartControl"
	TrayTooltipManualMode   Key = "tray.tooltip.manualMode"
	TrayTooltipFan          Key = "tray.tooltip.fan"
)

// 托盘菜单。*Tip 为菜单项的提示文本。
const (
	TrayMenuShow               Key = "tray.menu.show"
	TrayMenuShowTip            Key = "tray.menu.showTip"
	TrayMenuDeviceStatus       Key = "tray.menu.deviceStatus"
	TrayMenuDeviceStatusTip    Key = "tray.menu.deviceStatusTip"
	TrayMenuDeviceConnected    Key = "tray.menu.deviceConnected"
	TrayMenuDeviceDisconnected Key = "tray.menu.deviceDisconnected"
	TrayMenuCPUTemp            Key = "tray.menu.cpuTemp"
	TrayMenuCPUTempTip         Key = "tray.menu.cpuTempTip"
	TrayMenuCPUTempValue       Key = "tray.menu.cpuTempValue"
	TrayMenuCPUTempNoData      Key = "tray.menu.cpuTempNoData"
	TrayMenuGPUTemp            Key = "tray.menu.gpuTemp"
	TrayMenuGPUTempTip         Key = "tray.menu.gpuTempTip"
	TrayMenuGPUTempValue       Key = "tray.menu.gpuTempValue"
	TrayMenuGPUTempNoData      Key = "tray.menu.gpuTempNoData"
	TrayMenuCPUPower           Key = "tray.menu.cpuPower"
	TrayMenuCPUPowerTip        Key = "tray.menu.cpuPowerTip"
	TrayMenuCPUPowerValue      Key = "tray.menu.cpuPowerValue"
	TrayMenuGPUPower           Key = "tray.menu.gpuPower"
	TrayMenuGPUPowerTip        Key = "tray.menu.gpuPowerTip"
	TrayMenuGPUPowerValue      Key = "tray.menu.gpuPowerValue"
	TrayMenuFanSpeed           Key = "tray.menu.fanSpeed"
	TrayMenuFanSpeedTip        Key = "tray.menu.fanSpeedTip"
	TrayMenuFanSpeedValue      Key = "tray.menu.fanSpeedValue"
	TrayMenuFanSpeedNoData     Key = "tray.menu.fanSpeedNoData"
	TrayMenuCurveSelect        Key = "tray.menu.curveSelect"
	TrayMenuCurveSelectTip     Key = "tray.menu.curveSelectTip"
	TrayMenuCurveItemTip       Key = "tray.menu.curveItemTip"
	TrayMenuCurveEmpty         Key = "tray.menu.curveEmpty"
	TrayMenuAutoControl        Key = "tray.menu.autoControl"
	TrayMenuAutoControlTip     Key = "tray.menu.autoControlTip"
	TrayMenuQuit               Key = "tray.menu.quit"
	TrayMenuQuitTip            Key = "tray.menu.quitTip"
)

// 曲线方案、系统通知、全局快捷键。
const (
	CurveDefaultName Key = "curve.defaultName"

	NotifyDefaultTitle Key = "notify.defaultTitle"

	// NotifyDeviceUnresponsive 用于 BS1 走 USB 时"设备还在系统里但不再通信"的提醒。
	// 这种状态软件重连永远修不好，只有拔插 USB 线或重启系统能恢复。
	NotifyDeviceUnresponsiveTitle Key = "notify.deviceUnresponsiveTitle"
	NotifyDeviceUnresponsiveBody  Key = "notify.deviceUnresponsiveBody"

	HotkeyTitle                  Key = "hotkey.title"
	HotkeyFailedTitle            Key = "hotkey.failedTitle"
	HotkeyUnknownAction          Key = "hotkey.unknownAction"
	HotkeyCurveSwitched          Key = "hotkey.curveSwitched"
	HotkeyAutoOn                 Key = "hotkey.autoOn"
	HotkeyAutoOff                Key = "hotkey.autoOff"
	HotkeyManualGearRPM          Key = "hotkey.manualGearRpm"
	HotkeyManualGear             Key = "hotkey.manualGear"
	HotkeySwitchManualFailed     Key = "hotkey.switchManualFailed"
	HotkeyApplyGearFailed        Key = "hotkey.applyGearFailed"
	HotkeyNoCurveProfiles        Key = "hotkey.noCurveProfiles"
	HotkeyAutoBlockedByCustomRPM Key = "hotkey.autoBlockedByCustomRpm"
	HotkeyActionFailed           Key = "hotkey.actionFailed"
)

// 挡位与小挡位显示名（与前端 manualGear.gears / manualGear.levels 一致）。
const (
	GearQuiet     Key = "gear.quiet"
	GearStandard  Key = "gear.standard"
	GearStrong    Key = "gear.strong"
	GearOverclock Key = "gear.overclock"
	LevelLow      Key = "level.low"
	LevelMedium   Key = "level.medium"
	LevelHigh     Key = "level.high"
)

// catalog 的 zh-CN 部分必须与引入本包之前的硬编码逐字一致。
var catalog = map[string]map[Key]string{
	ZhCN: {
		TrayTooltipRunning:      "%s - 运行中",
		TrayTooltipDisconnected: "%s - 设备未连接",
		TrayTooltipSmartControl: "%s - 智能变频中",
		TrayTooltipManualMode:   "%s - 手动模式",
		TrayTooltipFan:          "风扇 %d RPM",

		TrayMenuShow:               "显示主窗口",
		TrayMenuShowTip:            "显示控制器主窗口",
		TrayMenuDeviceStatus:       "设备状态",
		TrayMenuDeviceStatusTip:    "查看设备连接状态",
		TrayMenuDeviceConnected:    "设备状态: 已连接",
		TrayMenuDeviceDisconnected: "设备状态: 未连接",
		TrayMenuCPUTemp:            "CPU温度",
		TrayMenuCPUTempTip:         "显示当前CPU温度",
		TrayMenuCPUTempValue:       "CPU温度: %d°C",
		TrayMenuCPUTempNoData:      "CPU温度: 无数据",
		TrayMenuGPUTemp:            "GPU温度",
		TrayMenuGPUTempTip:         "显示当前GPU温度",
		TrayMenuGPUTempValue:       "GPU温度: %d°C",
		TrayMenuGPUTempNoData:      "GPU温度: 无数据",
		TrayMenuCPUPower:           "CPU功耗",
		TrayMenuCPUPowerTip:        "显示当前CPU封装功耗",
		TrayMenuCPUPowerValue:      "CPU功耗: %.1f W",
		TrayMenuGPUPower:           "GPU功耗",
		TrayMenuGPUPowerTip:        "显示当前GPU功耗",
		TrayMenuGPUPowerValue:      "GPU功耗: %.1f W",
		TrayMenuFanSpeed:           "风扇转速",
		TrayMenuFanSpeedTip:        "显示当前风扇转速",
		TrayMenuFanSpeedValue:      "风扇转速: %d RPM",
		TrayMenuFanSpeedNoData:     "风扇转速: 无数据",
		TrayMenuCurveSelect:        "选择温控曲线",
		TrayMenuCurveSelectTip:     "直接切换到指定温控曲线",
		TrayMenuCurveItemTip:       "切换温控曲线",
		TrayMenuCurveEmpty:         "暂无可用曲线",
		TrayMenuAutoControl:        "智能变频",
		TrayMenuAutoControlTip:     "启用/禁用智能变频",
		TrayMenuQuit:               "退出",
		TrayMenuQuitTip:            "完全退出应用",

		CurveDefaultName: "默认",

		NotifyDefaultTitle: "功能变动",

		NotifyDeviceUnresponsiveTitle: "BS1 USB 通道无响应",
		NotifyDeviceUnresponsiveBody:  "设备仍在系统中，但已不再通信。请拔下 USB-C 线重新插入，或重启电脑。软件重连无法恢复。",

		HotkeyTitle:                  "%s 快捷键",
		HotkeyFailedTitle:            "%s 快捷键失败",
		HotkeyUnknownAction:          "未知快捷键动作",
		HotkeyCurveSwitched:          "温控曲线已切换: %s",
		HotkeyAutoOn:                 "智能变频已开启",
		HotkeyAutoOff:                "智能变频已关闭",
		HotkeyManualGearRPM:          "手动挡位: %s %s (%d RPM)",
		HotkeyManualGear:             "手动挡位: %s %s",
		HotkeySwitchManualFailed:     "切换到手动模式失败",
		HotkeyApplyGearFailed:        "应用手动挡位失败",
		HotkeyNoCurveProfiles:        "暂无可用温控曲线方案",
		HotkeyAutoBlockedByCustomRPM: "自定义转速模式下无法开启智能变频",
		HotkeyActionFailed:           "%s",

		GearQuiet:     "静音",
		GearStandard:  "标准",
		GearStrong:    "强劲",
		GearOverclock: "超频",
		LevelLow:      "低",
		LevelMedium:   "中",
		LevelHigh:     "高",
	},
	EnUS: {
		TrayTooltipRunning:      "%s - Running",
		TrayTooltipDisconnected: "%s - Device not connected",
		TrayTooltipSmartControl: "%s - Smart mode",
		TrayTooltipManualMode:   "%s - Manual mode",
		TrayTooltipFan:          "Fan %d RPM",

		TrayMenuShow:               "Show main window",
		TrayMenuShowTip:            "Show the controller main window",
		TrayMenuDeviceStatus:       "Device status",
		TrayMenuDeviceStatusTip:    "View device connection status",
		TrayMenuDeviceConnected:    "Device status: Connected",
		TrayMenuDeviceDisconnected: "Device status: Disconnected",
		TrayMenuCPUTemp:            "CPU temperature",
		TrayMenuCPUTempTip:         "Show the current CPU temperature",
		TrayMenuCPUTempValue:       "CPU temperature: %d°C",
		TrayMenuCPUTempNoData:      "CPU temperature: No data",
		TrayMenuGPUTemp:            "GPU temperature",
		TrayMenuGPUTempTip:         "Show the current GPU temperature",
		TrayMenuGPUTempValue:       "GPU temperature: %d°C",
		TrayMenuGPUTempNoData:      "GPU temperature: No data",
		TrayMenuCPUPower:           "CPU power",
		TrayMenuCPUPowerTip:        "Show the current CPU package power",
		TrayMenuCPUPowerValue:      "CPU power: %.1f W",
		TrayMenuGPUPower:           "GPU power",
		TrayMenuGPUPowerTip:        "Show the current GPU power",
		TrayMenuGPUPowerValue:      "GPU power: %.1f W",
		TrayMenuFanSpeed:           "Fan speed",
		TrayMenuFanSpeedTip:        "Show the current fan speed",
		TrayMenuFanSpeedValue:      "Fan speed: %d RPM",
		TrayMenuFanSpeedNoData:     "Fan speed: No data",
		TrayMenuCurveSelect:        "Select fan curve",
		TrayMenuCurveSelectTip:     "Switch directly to the selected fan curve",
		TrayMenuCurveItemTip:       "Switch fan curve",
		TrayMenuCurveEmpty:         "No curves available",
		TrayMenuAutoControl:        "Smart control",
		TrayMenuAutoControlTip:     "Enable/disable smart control",
		TrayMenuQuit:               "Exit",
		TrayMenuQuitTip:            "Quit the application completely",

		CurveDefaultName: "Default",

		NotifyDefaultTitle: "Feature updated",

		NotifyDeviceUnresponsiveTitle: "BS1 USB link unresponsive",
		NotifyDeviceUnresponsiveBody:  "The device is still present but no longer responding. Unplug and reinsert the USB-C cable, or restart the computer. Reconnecting in software cannot recover it.",

		HotkeyTitle:                  "%s hotkey",
		HotkeyFailedTitle:            "%s hotkey failed",
		HotkeyUnknownAction:          "Unknown hotkey action",
		HotkeyCurveSwitched:          "Temperature curve switched: %s",
		HotkeyAutoOn:                 "Smart control enabled",
		HotkeyAutoOff:                "Smart control disabled",
		HotkeyManualGearRPM:          "Manual gear: %s %s (%d RPM)",
		HotkeyManualGear:             "Manual gear: %s %s",
		HotkeySwitchManualFailed:     "Failed to switch to manual mode",
		HotkeyApplyGearFailed:        "Failed to apply manual gear",
		HotkeyNoCurveProfiles:        "No curve profiles available",
		HotkeyAutoBlockedByCustomRPM: "Smart control can't be enabled while custom speed is on",
		HotkeyActionFailed:           "Action failed: %s",

		GearQuiet:     "Quiet",
		GearStandard:  "Standard",
		GearStrong:    "Strong",
		GearOverclock: "Overclock",
		LevelLow:      "Low",
		LevelMedium:   "Medium",
		LevelHigh:     "High",
	},
	JaJP: {
		TrayTooltipRunning:      "%s - 実行中",
		TrayTooltipDisconnected: "%s - デバイス未接続",
		TrayTooltipSmartControl: "%s - スマート制御中",
		TrayTooltipManualMode:   "%s - 手動制御",
		TrayTooltipFan:          "ファン %d RPM",

		TrayMenuShow:               "メインウィンドウを表示",
		TrayMenuShowTip:            "コントローラーのメインウィンドウを表示します",
		TrayMenuDeviceStatus:       "デバイス状態",
		TrayMenuDeviceStatusTip:    "デバイスの接続状態を確認します",
		TrayMenuDeviceConnected:    "デバイス状態: 接続済み",
		TrayMenuDeviceDisconnected: "デバイス状態: 未接続",
		TrayMenuCPUTemp:            "CPU 温度",
		TrayMenuCPUTempTip:         "現在の CPU 温度を表示します",
		TrayMenuCPUTempValue:       "CPU 温度: %d°C",
		TrayMenuCPUTempNoData:      "CPU 温度: データなし",
		TrayMenuGPUTemp:            "GPU 温度",
		TrayMenuGPUTempTip:         "現在の GPU 温度を表示します",
		TrayMenuGPUTempValue:       "GPU 温度: %d°C",
		TrayMenuGPUTempNoData:      "GPU 温度: データなし",
		TrayMenuCPUPower:           "CPU 消費電力",
		TrayMenuCPUPowerTip:        "現在の CPU パッケージ消費電力を表示します",
		TrayMenuCPUPowerValue:      "CPU 消費電力: %.1f W",
		TrayMenuGPUPower:           "GPU 消費電力",
		TrayMenuGPUPowerTip:        "現在の GPU 消費電力を表示します",
		TrayMenuGPUPowerValue:      "GPU 消費電力: %.1f W",
		TrayMenuFanSpeed:           "ファン回転数",
		TrayMenuFanSpeedTip:        "現在のファン回転数を表示します",
		TrayMenuFanSpeedValue:      "ファン回転数: %d RPM",
		TrayMenuFanSpeedNoData:     "ファン回転数: データなし",
		TrayMenuCurveSelect:        "ファンカーブを選択",
		TrayMenuCurveSelectTip:     "指定したファンカーブに直接切り替えます",
		TrayMenuCurveItemTip:       "ファンカーブを切り替え",
		TrayMenuCurveEmpty:         "利用できるカーブがありません",
		TrayMenuAutoControl:        "スマート制御",
		TrayMenuAutoControlTip:     "スマート制御を有効/無効にします",
		TrayMenuQuit:               "終了",
		TrayMenuQuitTip:            "アプリケーションを完全に終了します",

		CurveDefaultName: "デフォルト",

		NotifyDefaultTitle: "機能が更新されました",

		NotifyDeviceUnresponsiveTitle: "BS1 USB 接続が応答しません",
		NotifyDeviceUnresponsiveBody:  "デバイスは認識されていますが通信できません。USB-C ケーブルを抜き差しするか、PC を再起動してください。ソフト側の再接続では復旧しません。",

		HotkeyTitle:                  "%s ショートカット",
		HotkeyFailedTitle:            "%s ショートカット失敗",
		HotkeyUnknownAction:          "不明なショートカット操作です",
		HotkeyCurveSwitched:          "温度カーブを切り替えました: %s",
		HotkeyAutoOn:                 "スマート制御を有効にしました",
		HotkeyAutoOff:                "スマート制御を無効にしました",
		HotkeyManualGearRPM:          "マニュアル: %s %s (%d RPM)",
		HotkeyManualGear:             "マニュアル: %s %s",
		HotkeySwitchManualFailed:     "手動制御への切り替えに失敗しました",
		HotkeyApplyGearFailed:        "マニュアルギアの適用に失敗しました",
		HotkeyNoCurveProfiles:        "利用できるカーブプロファイルがありません",
		HotkeyAutoBlockedByCustomRPM: "カスタム回転数が有効な間はスマート制御を有効にできません",
		HotkeyActionFailed:           "操作に失敗しました: %s",

		GearQuiet:     "サイレント",
		GearStandard:  "スタンダード",
		GearStrong:    "ストロング",
		GearOverclock: "オーバークロック",
		LevelLow:      "低",
		LevelMedium:   "中",
		LevelHigh:     "高",
	},
	KoKR: {
		TrayTooltipRunning:      "%s - 실행 중",
		TrayTooltipDisconnected: "%s - 장치 연결 안 됨",
		TrayTooltipSmartControl: "%s - 스마트 제어 중",
		TrayTooltipManualMode:   "%s - 수동 모드",
		TrayTooltipFan:          "팬 %d RPM",

		TrayMenuShow:               "메인 창 표시",
		TrayMenuShowTip:            "컨트롤러 메인 창을 표시합니다",
		TrayMenuDeviceStatus:       "장치 상태",
		TrayMenuDeviceStatusTip:    "장치 연결 상태를 확인합니다",
		TrayMenuDeviceConnected:    "장치 상태: 연결됨",
		TrayMenuDeviceDisconnected: "장치 상태: 연결 안 됨",
		TrayMenuCPUTemp:            "CPU 온도",
		TrayMenuCPUTempTip:         "현재 CPU 온도를 표시합니다",
		TrayMenuCPUTempValue:       "CPU 온도: %d°C",
		TrayMenuCPUTempNoData:      "CPU 온도: 데이터 없음",
		TrayMenuGPUTemp:            "GPU 온도",
		TrayMenuGPUTempTip:         "현재 GPU 온도를 표시합니다",
		TrayMenuGPUTempValue:       "GPU 온도: %d°C",
		TrayMenuGPUTempNoData:      "GPU 온도: 데이터 없음",
		TrayMenuCPUPower:           "CPU 전력",
		TrayMenuCPUPowerTip:        "현재 CPU 패키지 전력을 표시합니다",
		TrayMenuCPUPowerValue:      "CPU 전력: %.1f W",
		TrayMenuGPUPower:           "GPU 전력",
		TrayMenuGPUPowerTip:        "현재 GPU 전력을 표시합니다",
		TrayMenuGPUPowerValue:      "GPU 전력: %.1f W",
		TrayMenuFanSpeed:           "팬 속도",
		TrayMenuFanSpeedTip:        "현재 팬 속도를 표시합니다",
		TrayMenuFanSpeedValue:      "팬 속도: %d RPM",
		TrayMenuFanSpeedNoData:     "팬 속도: 데이터 없음",
		TrayMenuCurveSelect:        "팬 커브 선택",
		TrayMenuCurveSelectTip:     "선택한 팬 커브로 바로 전환합니다",
		TrayMenuCurveItemTip:       "팬 커브 전환",
		TrayMenuCurveEmpty:         "사용 가능한 커브가 없습니다",
		TrayMenuAutoControl:        "스마트 제어",
		TrayMenuAutoControlTip:     "스마트 제어 켜기/끄기",
		TrayMenuQuit:               "종료",
		TrayMenuQuitTip:            "애플리케이션을 완전히 종료합니다",

		CurveDefaultName: "기본",

		NotifyDefaultTitle: "기능이 변경되었습니다",

		NotifyDeviceUnresponsiveTitle: "BS1 USB 연결 응답 없음",
		NotifyDeviceUnresponsiveBody:  "장치는 시스템에 인식되어 있지만 더 이상 통신하지 않습니다. USB-C 케이블을 뽑았다가 다시 꽂거나 PC를 재시작하세요. 소프트웨어 재연결로는 복구되지 않습니다.",

		HotkeyTitle:                  "%s 단축키",
		HotkeyFailedTitle:            "%s 단축키 실패",
		HotkeyUnknownAction:          "알 수 없는 단축키 동작",
		HotkeyCurveSwitched:          "온도 커브 전환됨: %s",
		HotkeyAutoOn:                 "스마트 제어를 켰습니다",
		HotkeyAutoOff:                "스마트 제어를 껐습니다",
		HotkeyManualGearRPM:          "수동 단계: %s %s (%d RPM)",
		HotkeyManualGear:             "수동 단계: %s %s",
		HotkeySwitchManualFailed:     "수동 모드로 전환하지 못했습니다",
		HotkeyApplyGearFailed:        "수동 단계를 적용하지 못했습니다",
		HotkeyNoCurveProfiles:        "사용 가능한 커브 프로필이 없습니다",
		HotkeyAutoBlockedByCustomRPM: "사용자 지정 속도 모드에서는 스마트 제어를 켤 수 없습니다",
		HotkeyActionFailed:           "작업 실패: %s",

		GearQuiet:     "저소음",
		GearStandard:  "표준",
		GearStrong:    "강력",
		GearOverclock: "오버클럭",
		LevelLow:      "낮음",
		LevelMedium:   "중간",
		LevelHigh:     "높음",
	},
}
