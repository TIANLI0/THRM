//go:build windows

package flydigicompat

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestCoolerNodeRegexMatchesCoolers(t *testing.T) {
	cases := []string{
		// BLE 接入（BS3PRO）
		"{00001812-0000-1000-8000-00805F9B34FB}_DEV_VID&0137D7_PID&1004_REV&0110_DC7F642D6C29",
		// 大小写混用，实际注册表里两种都出现过
		"{00001812-0000-1000-8000-00805f9b34fb}_Dev_VID&0137d7_PID&1004_REV&0110_dc7f642d6c29",
		// USB 接入
		"VID_37D7&PID_1002&MI_00",
		"VID_37D7&PID_1001",
		"VID_37D7&PID_1003&MI_01&COL02",
	}
	for _, name := range cases {
		if !coolerNodeRe.MatchString(name) {
			t.Errorf("散热器节点未被匹配: %s", name)
		}
	}
}

func TestCoolerNodeRegexIgnoresOtherDevices(t *testing.T) {
	cases := []string{
		// 飞智手柄用的是别的产品号，绝不能被误伤
		"VID_37D7&PID_1010&MI_00",
		"VID_37D7&PID_2004",
		"{00001812-0000-1000-8000-00805F9B34FB}_DEV_VID&0137D7_PID&0B22_REV&0522_44162232CFDC",
		// 别家的设备
		"VID_045E&PID_1004&MI_00",
		"VID_24AE&PID_1418&MI_01&COL03",
		"INTC816&COL02",
	}
	for _, name := range cases {
		if coolerNodeRe.MatchString(name) {
			t.Errorf("非散热器节点被误匹配: %s", name)
		}
	}
}

func TestNodeRefPaths(t *testing.T) {
	n := nodeRef{
		device:   "{00001812-0000-1000-8000-00805F9B34FB}_DEV_VID&0137D7_PID&1004_REV&0110_DC7F642D6C29",
		instance: "9&2D4529BA&0&0000",
	}

	wantInstance := `HID\{00001812-0000-1000-8000-00805F9B34FB}_DEV_VID&0137D7_PID&1004_REV&0110_DC7F642D6C29\9&2D4529BA&0&0000`
	if got := n.instanceID(); got != wantInstance {
		t.Errorf("instanceID() = %q, 期望 %q", got, wantInstance)
	}

	wantReg := `SYSTEM\CurrentControlSet\Enum\` + wantInstance
	if got := n.regPath(); got != wantReg {
		t.Errorf("regPath() = %q, 期望 %q", got, wantReg)
	}

	wantIface := `\\?\HID#{00001812-0000-1000-8000-00805F9B34FB}_DEV_VID&0137D7_PID&1004_REV&0110_DC7F642D6C29#9&2D4529BA&0&0000#{4d1e55b2-f16f-11cf-88cb-001111000030}`
	if got := n.interfacePath(); got != wantIface {
		t.Errorf("interfacePath() = %q, 期望 %q", got, wantIface)
	}
}

func TestDesiredSecurityBytesIsSelfRelative(t *testing.T) {
	got, err := desiredSecurityBytes()
	if err != nil {
		// 以 SYSTEM 运行测试时会走保护性报错，这是预期行为。
		if err.Error() == ErrRunningAsSystem {
			t.Skip("测试进程以 SYSTEM 运行，跳过")
		}
		t.Fatalf("构造安全描述符失败: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("安全描述符为空")
	}
	// 自相对安全描述符的 Control 字段(偏移 2..3, 小端)必须带 SE_SELF_RELATIVE(0x8000)。
	control := uint16(got[2]) | uint16(got[3])<<8
	if control&0x8000 == 0 {
		t.Errorf("安全描述符不是自相对格式: control=0x%04X", control)
	}
}

// TestDesiredSecurityDeniesSystem 守住这个功能的核心前提：
// LocalSystem 的令牌里含 BUILTIN\Administrators 组，所以"不授予 SY"挡不住飞智服务，
// 必须有一条显式的拒绝 ACE，且排在允许 ACE 之前。
func TestDesiredSecurityDeniesSystem(t *testing.T) {
	raw, err := desiredSecurityBytes()
	if err != nil {
		if err.Error() == ErrRunningAsSystem {
			t.Skip("测试进程以 SYSTEM 运行，跳过")
		}
		t.Fatalf("构造安全描述符失败: %v", err)
	}

	sd, err := windows.SecurityDescriptorFromString(sddlFromBytes(t, raw))
	if err != nil {
		t.Fatalf("回读安全描述符失败: %v", err)
	}
	sddl := strings.ToUpper(sd.String())

	if !syDenyACERe.MatchString(sddl) {
		t.Errorf("缺少拒绝 SYSTEM 的 ACE: %s", sddl)
	}
	if syAllowACERe.MatchString(sddl) {
		t.Errorf("不应存在允许 SYSTEM 的 ACE: %s", sddl)
	}

	denyIdx := strings.Index(sddl, "(D;")
	allowIdx := strings.Index(sddl, "(A;")
	if denyIdx < 0 || allowIdx < 0 || denyIdx > allowIdx {
		t.Errorf("拒绝 ACE 必须排在允许 ACE 之前: %s", sddl)
	}
}

func sddlFromBytes(t *testing.T, raw []byte) string {
	t.Helper()
	sd := (*windows.SECURITY_DESCRIPTOR)(unsafe.Pointer(&raw[0]))
	return sd.String()
}

// TestDetectDoesNotPanic 是只读冒烟测试：Detect 不应修改任何东西。
func TestDetectDoesNotPanic(t *testing.T) {
	st := Detect(nil)
	effective := "未知(无在线设备)"
	if st.Effective != nil {
		effective = fmt.Sprintf("%v", *st.Effective)
	}
	t.Logf("状态: 服务已装=%v 服务运行中=%v 节点=%d 已写入=%d 在线=%d 生效=%s 需重连=%v 错误=%q",
		st.ServiceInstalled, st.ServiceRunning, st.TotalNodes, st.AppliedNodes,
		st.PresentNodes, effective, st.NeedsReconnect, st.Error)
	if !st.Supported {
		t.Error("Windows 上 Supported 应为 true")
	}
	if st.AppliedNodes > st.TotalNodes {
		t.Errorf("AppliedNodes(%d) 不应大于 TotalNodes(%d)", st.AppliedNodes, st.TotalNodes)
	}
	if st.PresentNodes > st.TotalNodes {
		t.Errorf("PresentNodes(%d) 不应大于 TotalNodes(%d)", st.PresentNodes, st.TotalNodes)
	}
}
