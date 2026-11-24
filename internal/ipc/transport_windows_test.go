//go:build windows

package ipc

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPipeSecurityDescriptor(t *testing.T) {
	sddl := pipeSecurityDescriptor()
	if _, err := windows.SecurityDescriptorFromString(sddl); err != nil {
		t.Fatalf("SDDL %q 无法解析: %v", sddl, err)
	}
	// Everyone / 匿名 / 网络登录都不应出现在 ACL 里。
	for _, sid := range []string{";WD)", ";AN)", ";NU)", ";AU)", ";BU)"} {
		if strings.Contains(sddl, sid) {
			t.Errorf("SDDL %q 不应包含 %s", sddl, sid)
		}
	}
	if !strings.HasPrefix(sddl, "D:P(A;;GA;;;SY)(A;;GA;;;BA)") {
		t.Errorf("SDDL %q 应以 SYSTEM 与 Administrators 开头", sddl)
	}
	if windows.GetCurrentProcessToken().IsElevated() {
		if sddl != "D:P(A;;GA;;;SY)(A;;GA;;;BA)" {
			t.Errorf("已提权时只应允许 SYSTEM 与 Administrators，得到 %q", sddl)
		}
		return
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sddl, user.User.Sid.String()) {
		t.Errorf("未提权时应包含当前用户 SID，得到 %q", sddl)
	}
}
