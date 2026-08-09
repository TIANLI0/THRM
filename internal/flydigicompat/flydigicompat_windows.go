//go:build windows

package flydigicompat

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unsafe"

	"github.com/TIANLI0/THRM/internal/types"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	enumHIDPath    = `SYSTEM\CurrentControlSet\Enum\HID`
	serviceKeyPath = `SYSTEM\CurrentControlSet\Services\Flydigi Space Station Service`
	serviceName    = "Flydigi Space Station Service"

	// securityValueName 即 SPDRP_SECURITY，设备对象创建时会用它作为安全描述符。
	securityValueName = "Security"

	// hidInterfaceGUID 是 GUID_DEVINTERFACE_HID，用于拼出设备接口路径。
	hidInterfaceGUID = "{4d1e55b2-f16f-11cf-88cb-001111000030}"

	backupFileName = "flydigi-compat-backup.json"

	localSystemSID = "S-1-5-18"
)

// coolerNodeRe 匹配散热器设备节点名。
//
// BLE 接入形如  {00001812-...}_DEV_VID&0137D7_PID&1004_REV&0110_XXXX
// USB 接入形如  VID_37D7&PID_1002&MI_00
//
// 产品号只取 1001..1004（BS2 / BS2PRO / BS3 / BS3PRO），手柄是别的 PID，不会被误伤。
var coolerNodeRe = regexp.MustCompile(`(?i)VID[&_]0?1?37D7.{0,6}PID[&_]0?100[1-4]`)

// applyMutex 串行化写操作：健康检查循环和 IPC 请求可能同时触发。
var applyMutex sync.Mutex

// nodeRef 指向一个散热器 HID 设备实例节点。
type nodeRef struct {
	device   string // 设备键名，如 {00001812-...}_DEV_VID&0137D7_PID&1004_REV&0110_XXXX
	instance string // 实例键名，如 9&2d4529ba&0&0000
}

func (n nodeRef) instanceID() string { return `HID\` + n.device + `\` + n.instance }

func (n nodeRef) regPath() string { return enumHIDPath + `\` + n.device + `\` + n.instance }

// interfacePath 拼出 \\?\HID#...#{GUID} 形式的设备接口路径。
func (n nodeRef) interfacePath() string {
	return `\\?\` + strings.ReplaceAll(n.instanceID(), `\`, `#`) + `#` + hidInterfaceGUID
}

// backupEntry 记录某个节点在 THRM 改动之前的 Security 值。
type backupEntry struct {
	InstanceID string `json:"instanceId"`
	HadValue   bool   `json:"hadValue"`
	Value      string `json:"value"` // base64；HadValue 为 false 时为空
}

// Detect 检测当前状态，不做任何修改。
func Detect(logger types.Logger) Status {
	st := Status{Supported: true}
	st.ServiceInstalled, st.ServiceRunning = serviceState()

	nodes, err := enumerateCoolerNodes()
	if err != nil {
		st.Error = fmt.Sprintf("枚举散热器设备节点失败: %v", err)
		return st
	}
	st.TotalNodes = len(nodes)
	if len(nodes) == 0 {
		return st
	}

	want, err := desiredSecurityBytes()
	if err != nil {
		st.Error = err.Error()
		return st
	}

	var effectiveKnown, effectiveAll bool
	effectiveAll = true
	for _, n := range nodes {
		if nodeHasSecurity(n, want) {
			st.AppliedNodes++
		}
		present, effective := nodeLiveState(n)
		if !present {
			continue
		}
		st.PresentNodes++
		effectiveKnown = true
		if !effective {
			effectiveAll = false
		}
	}

	if effectiveKnown {
		v := effectiveAll
		st.Effective = &v
		st.NeedsReconnect = st.AppliedNodes > 0 && !effectiveAll
	}

	if logger != nil {
		logger.Debug("飞智兼容检测: 节点=%d 已写入=%d 在线=%d 需重连=%v",
			st.TotalNodes, st.AppliedNodes, st.PresentNodes, st.NeedsReconnect)
	}
	return st
}

// NeedsApply 只扫注册表，判断是否存在尚未写入安全描述符的散热器设备节点。
//
// 供 30 秒健康检查循环调用：它不打开设备句柄，因此不会周期性地去戳散热器。
func NeedsApply(logger types.Logger) bool {
	nodes, err := enumerateCoolerNodes()
	if err != nil {
		if logger != nil {
			logger.Debug("枚举散热器设备节点失败: %v", err)
		}
		return false
	}
	if len(nodes) == 0 {
		return false
	}

	want, err := desiredSecurityBytes()
	if err != nil {
		return false
	}
	for _, n := range nodes {
		if !nodeHasSecurity(n, want) {
			return true
		}
	}
	return false
}

// Apply 给所有散热器设备节点写入 THRM 的安全描述符，把飞智服务(LocalSystem)挡在门外。
//
// 只写注册表，绝不 disable / enable 设备——那会把设备留在 Code 22 禁用状态。
// 安全描述符在设备对象重新创建时生效，也就是散热器重新连接或系统重启之后。
func Apply(logger types.Logger, stateDir string) (Status, error) {
	applyMutex.Lock()
	defer applyMutex.Unlock()

	if !isRunningAsAdmin() {
		return Detect(logger), errors.New(ErrNeedsAdmin)
	}

	want, err := desiredSecurityBytes()
	if err != nil {
		return Detect(logger), err
	}

	nodes, err := enumerateCoolerNodes()
	if err != nil {
		return Detect(logger), fmt.Errorf("枚举散热器设备节点失败: %w", err)
	}
	if len(nodes) == 0 {
		// 散热器从没连过这台机器，没有节点可写。等设备接入后由自动兼容处理补上。
		return Detect(logger), nil
	}

	backup := loadBackup(stateDir)
	changed := 0

	for _, n := range nodes {
		if nodeHasSecurity(n, want) {
			continue
		}

		key, err := registry.OpenKey(registry.LOCAL_MACHINE, n.regPath(),
			registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			if logger != nil {
				logger.Warn("打开设备节点失败 %s: %v", n.instanceID(), err)
			}
			continue
		}

		// 只在第一次改动该节点时记录原值，避免把 THRM 自己写的值当成原值备份。
		if _, recorded := backup[n.instanceID()]; !recorded {
			entry := backupEntry{InstanceID: n.instanceID()}
			if old, _, err := key.GetBinaryValue(securityValueName); err == nil {
				entry.HadValue = true
				entry.Value = base64.StdEncoding.EncodeToString(old)
			}
			backup[n.instanceID()] = entry
		}

		err = key.SetBinaryValue(securityValueName, want)
		key.Close()
		if err != nil {
			if logger != nil {
				logger.Error("写入设备安全描述符失败 %s: %v", n.instanceID(), err)
			}
			continue
		}
		changed++
		if logger != nil {
			logger.Info("已为散热器设备节点写入安全描述符: %s", n.instanceID())
		}
	}

	if changed > 0 {
		if err := saveBackup(stateDir, backup); err != nil && logger != nil {
			logger.Warn("保存飞智兼容备份失败: %v", err)
		}
	}

	return Detect(logger), nil
}

// Revert 撤销 Apply 的改动，按备份还原（原本没有该值就删除）。
func Revert(logger types.Logger, stateDir string) (Status, error) {
	applyMutex.Lock()
	defer applyMutex.Unlock()

	if !isRunningAsAdmin() {
		return Detect(logger), errors.New(ErrNeedsAdmin)
	}

	backup := loadBackup(stateDir)

	nodes, err := enumerateCoolerNodes()
	if err != nil {
		return Detect(logger), fmt.Errorf("枚举散热器设备节点失败: %w", err)
	}

	want, wantErr := desiredSecurityBytes()

	for _, n := range nodes {
		entry, recorded := backup[n.instanceID()]
		// 没有备份记录时，只有当值确实是 THRM 写的才动它，避免误删别人的安全描述符。
		if !recorded {
			if wantErr != nil || !nodeHasSecurity(n, want) {
				continue
			}
			entry = backupEntry{InstanceID: n.instanceID(), HadValue: false}
		}

		key, err := registry.OpenKey(registry.LOCAL_MACHINE, n.regPath(),
			registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			if logger != nil {
				logger.Warn("打开设备节点失败 %s: %v", n.instanceID(), err)
			}
			continue
		}

		if entry.HadValue {
			if old, decErr := base64.StdEncoding.DecodeString(entry.Value); decErr == nil {
				err = key.SetBinaryValue(securityValueName, old)
			} else {
				err = key.DeleteValue(securityValueName)
			}
		} else {
			err = key.DeleteValue(securityValueName)
			if errors.Is(err, registry.ErrNotExist) {
				err = nil
			}
		}
		key.Close()

		if err != nil {
			if logger != nil {
				logger.Error("还原设备安全描述符失败 %s: %v", n.instanceID(), err)
			}
			continue
		}
		delete(backup, n.instanceID())
		if logger != nil {
			logger.Info("已还原散热器设备节点安全描述符: %s", n.instanceID())
		}
	}

	if len(backup) == 0 {
		_ = os.Remove(filepath.Join(stateDir, backupFileName))
	} else if err := saveBackup(stateDir, backup); err != nil && logger != nil {
		logger.Warn("保存飞智兼容备份失败: %v", err)
	}

	return Detect(logger), nil
}

// enumerateCoolerNodes 遍历 Enum\HID，找出所有散热器设备实例节点（含当前不在线的）。
func enumerateCoolerNodes() ([]nodeRef, error) {
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, enumHIDPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	devices, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var nodes []nodeRef
	for _, device := range devices {
		if !coolerNodeRe.MatchString(device) {
			continue
		}
		devKey, err := registry.OpenKey(registry.LOCAL_MACHINE,
			enumHIDPath+`\`+device, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		instances, err := devKey.ReadSubKeyNames(-1)
		devKey.Close()
		if err != nil {
			continue
		}
		for _, instance := range instances {
			nodes = append(nodes, nodeRef{device: device, instance: instance})
		}
	}
	return nodes, nil
}

// nodeHasSecurity 判断节点上的 Security 值是否已经是 THRM 期望的那个。
func nodeHasSecurity(n nodeRef, want []byte) bool {
	if len(want) == 0 {
		return false
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, n.regPath(), registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	got, _, err := key.GetBinaryValue(securityValueName)
	if err != nil {
		return false
	}
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// nodeLiveState 打开在线设备对象，读它真实的 DACL，判断安全描述符是否已经生效。
//
// 注册表里的值只在设备对象创建时被读取，所以「写了」不等于「生效了」。
func nodeLiveState(n nodeRef) (present bool, effective bool) {
	path, err := windows.UTF16PtrFromString(n.interfacePath())
	if err != nil {
		return false, false
	}

	handle, err := windows.CreateFile(path, windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			// 设备在线，但连 READ_CONTROL 都拿不到——DACL 收紧了，且把 THRM 也挡了。
			return true, false
		}
		return false, false
	}
	defer windows.CloseHandle(handle)

	sd, err := windows.GetSecurityInfo(handle, windows.SE_KERNEL_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return true, false
	}

	// 生效的判据是设备对象上确实存在一条拒绝 SY 的 ACE，且没有允许 SY 的 ACE。
	//
	// 不能拿 SDDL 原文和期望值做字符串比较：写进去的通用权限(GA/GR/GW)在设备对象创建时
	// 会被映射成具体权限，回读出来是 0x1201bf 这样的数值。
	sddl := strings.ToUpper(sd.String())
	return true, syDenyACERe.MatchString(sddl) && !syAllowACERe.MatchString(sddl)
}

// ACE 串的格式是 (类型;标志;权限;对象GUID;继承对象GUID;账户SID)。
var (
	syDenyACERe  = regexp.MustCompile(`\(D;[^)]*;SY\)`)
	syAllowACERe = regexp.MustCompile(`\(A;[^)]*;SY\)`)
)

// desiredSecurityBytes 构造 THRM 期望的安全描述符（自相对格式的二进制）。
func desiredSecurityBytes() ([]byte, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, fmt.Errorf("获取当前用户 SID 失败: %w", err)
	}
	if sid == localSystemSID {
		return nil, errors.New(ErrRunningAsSystem)
	}

	// 关键：仅仅"不授予 SY"是不够的——LocalSystem 的令牌里含 BUILTIN\Administrators 组，
	// 会顺着 BA 那条 ACE 拿到访问权。必须放一条显式的拒绝 ACE，且排在最前面
	// （拒绝 ACE 先于允许 ACE 求值）。
	//
	// D:P 表示保护型 DACL，不继承。
	sddl := fmt.Sprintf("O:BAG:BAD:P(D;;GA;;;SY)(A;;GA;;;BA)(A;;GRGWGX;;;IU)(A;;GRGWGX;;;%s)", sid)

	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("构造安全描述符失败: %w", err)
	}
	// 注册表里存的必须是自相对格式。ConvertStringSecurityDescriptorToSecurityDescriptor
	// 本就返回自相对，这里只是兜底。
	if control, _, ctlErr := sd.Control(); ctlErr == nil && control&windows.SE_SELF_RELATIVE == 0 {
		sd, err = sd.ToSelfRelative()
		if err != nil {
			return nil, fmt.Errorf("转换安全描述符失败: %w", err)
		}
	}

	length := sd.Length()
	out := make([]byte, length)
	copy(out, unsafe.Slice((*byte)(unsafe.Pointer(sd)), length))
	return out, nil
}

// currentUserSID 取当前进程令牌的用户 SID。
func currentUserSID() (string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

// isRunningAsAdmin 判断当前进程令牌是否属于 Administrators 组。
func isRunningAsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	member, err := windows.Token(0).IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

// serviceState 查飞智空间站服务是否安装、是否在运行。
func serviceState() (installed bool, running bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, serviceKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, false
	}
	key.Close()
	installed = true

	// 只申请连接和查询状态的权限：mgr.Connect 要的是 SC_MANAGER_ALL_ACCESS，
	// 非管理员进程会直接失败，导致"服务在跑"被误报成"没在跑"。
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return installed, false
	}
	defer windows.CloseServiceHandle(scm)

	namePtr, err := windows.UTF16PtrFromString(serviceName)
	if err != nil {
		return installed, false
	}
	service, err := windows.OpenService(scm, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return installed, false
	}
	defer windows.CloseServiceHandle(service)

	var status windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(service, &status); err != nil {
		return installed, false
	}
	return installed, status.CurrentState == windows.SERVICE_RUNNING
}

func loadBackup(stateDir string) map[string]backupEntry {
	out := make(map[string]backupEntry)
	if stateDir == "" {
		return out
	}
	data, err := os.ReadFile(filepath.Join(stateDir, backupFileName))
	if err != nil {
		return out
	}
	var entries []backupEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return out
	}
	for _, e := range entries {
		out[e.InstanceID] = e
	}
	return out
}

func saveBackup(stateDir string, backup map[string]backupEntry) error {
	if stateDir == "" {
		return errors.New("备份目录为空")
	}
	entries := make([]backupEntry, 0, len(backup))
	for _, e := range backup {
		entries = append(entries, e)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, backupFileName), data, 0o644)
}
