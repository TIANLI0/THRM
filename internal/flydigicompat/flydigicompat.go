// Package flydigicompat 让 THRM 与飞智空间站(Flydigi Space Station)共存。
//
// 飞智空间站的后台服务 SpaceStationService.exe 以 LocalSystem 身份开机自启，会持续
// 向散热器下发 SetFanSpeed / SetRgbSmartMode，和 THRM 抢同一个 HID 接口。两者用的是
// 同一个设备，没法按设备拆分；但可以按账户拆分——飞智服务是 LocalSystem，THRM 是
// 当前登录用户。
//
// 于是本包给散热器的 HID 设备节点写入 Security 安全描述符（SPDRP_SECURITY，
// 存放在 HKLM\SYSTEM\CurrentControlSet\Enum\HID\<设备>\<实例> 的 Security 值），
// DACL 只授予 Administrators、交互用户和当前用户，不授予 SYSTEM。飞智服务因此打不开
// 散热器，而它的手柄映射、虚拟键鼠、扳机等功能全部不受影响。
//
// 只匹配散热器产品号（BS2/BS2PRO/BS3/BS3PRO），手柄用的是别的 PID，不在范围内。
//
// 注意：安全描述符是在设备对象被创建时生效的，写入注册表后必须等设备重新枚举
// （散热器重新连接或系统重启）才会真正起作用。Status.Effective 反映的是当前在线
// 设备对象的实际状态，而不是注册表里写了什么。
package flydigicompat

// Status 描述兼容处理的当前状态。字段带 json tag，直接透传给前端。
type Status struct {
	// Supported 当前平台是否支持（仅 Windows）
	Supported bool `json:"supported"`
	// ServiceInstalled 是否装了飞智空间站服务
	ServiceInstalled bool `json:"serviceInstalled"`
	// ServiceRunning 飞智空间站服务是否正在运行
	ServiceRunning bool `json:"serviceRunning"`
	// TotalNodes 注册表里匹配到的散热器设备节点数
	TotalNodes int `json:"totalNodes"`
	// AppliedNodes 已写入 THRM 安全描述符的节点数
	AppliedNodes int `json:"appliedNodes"`
	// PresentNodes 当前在线（设备对象存在）的节点数
	PresentNodes int `json:"presentNodes"`
	// Effective 在线设备对象上安全描述符是否已经真正生效。
	// nil 表示当前没有在线设备，无从判断。
	Effective *bool `json:"effective"`
	// NeedsReconnect 已写入注册表但在线设备对象上尚未生效，需要重连散热器或重启系统
	NeedsReconnect bool `json:"needsReconnect"`
	// Error 检测过程中的错误描述，空表示正常
	Error string `json:"error"`
}

// ErrRunningAsSystem 当 THRM 自身以 LocalSystem 运行时返回。
// 这种情况下写入这个安全描述符会把 THRM 自己也挡在门外。
const ErrRunningAsSystem = "THRM 正以 SYSTEM 身份运行，启用该兼容处理会把 THRM 自己也挡在门外"

// ErrNeedsAdmin 权限不足时返回。
const ErrNeedsAdmin = "写入设备安全描述符需要管理员权限"
