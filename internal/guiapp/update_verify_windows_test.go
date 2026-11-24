//go:build windows

package guiapp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// 测试二进制本身没有签名，正好当作"未签名安装包"。
func TestVerifyUpdateInstallerRejectsUnsigned(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyUpdateInstaller(self, ""); !errors.Is(err, errInstallerUnsigned) {
		t.Fatalf("verifyUpdateInstaller(unsigned) = %v, want errInstallerUnsigned", err)
	}
}

// 系统自带的 Microsoft 签名程序签名有效，但发布者不是 THRM：
// 当前程序未签名时只能放行，当前程序已签名时必须因发布者不一致被拒。
func TestVerifyUpdateInstallerPublisher(t *testing.T) {
	sysDir, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	// 多数系统文件走目录签名（catalog），没有内嵌签名；挑有内嵌签名的。
	var msSigned string
	for _, name := range []string{`..\explorer.exe`, "msiexec.exe", "consent.exe"} {
		p := filepath.Join(sysDir, name)
		if verifyAuthenticode(p) == nil {
			if _, err := authenticodeSigner(p); err == nil {
				msSigned = p
				break
			}
		}
	}
	if msSigned == "" {
		t.Skip("没有找到带内嵌签名的系统程序")
	}

	signer, err := authenticodeSigner(msSigned)
	if err != nil {
		t.Fatal(err)
	}
	if signer.CommonName == "" {
		t.Fatalf("签名者 CommonName 为空: %+v", signer)
	}
	if err := verifyUpdateInstaller(msSigned, msSigned); err != nil {
		t.Errorf("签名者相同时应通过: %v", err)
	}

	// THRM_TEST_SIGNED_EXE 指向一个 THRM 正式签名的程序时，验证跨发布者会被拒。
	thrmSigned := os.Getenv("THRM_TEST_SIGNED_EXE")
	if thrmSigned == "" {
		t.Log("未设置 THRM_TEST_SIGNED_EXE，跳过跨发布者检查")
		return
	}
	if err := verifyUpdateInstaller(thrmSigned, thrmSigned); err != nil {
		t.Errorf("THRM 签名的安装包应通过: %v", err)
	}
	if err := verifyUpdateInstaller(msSigned, thrmSigned); !errors.Is(err, errInstallerPublisher) {
		t.Errorf("发布者不一致应被拒, got %v", err)
	}
	s, _ := authenticodeSigner(thrmSigned)
	t.Logf("THRM signer: %s; system signer: %s", s, signer)
}

func TestCreateUpdateDir(t *testing.T) {
	dir, err := createUpdateDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "probe"), []byte("x"), 0o644); err != nil {
		t.Fatalf("更新目录不可写: %v", err)
	}
	other, err := createUpdateDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(other)
	if other == dir {
		t.Fatal("两次创建得到了同一个目录")
	}
}
