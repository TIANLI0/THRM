package guiapp

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNormalizeSHA256(t *testing.T) {
	sum := sha256.Sum256([]byte("thrm"))
	valid := hex.EncodeToString(sum[:])

	got, ok := normalizeSHA256("  " + strings.ToUpper(valid) + "\n")
	if !ok {
		t.Fatal("normalizeSHA256 rejected a valid digest with surrounding whitespace and uppercase")
	}
	if got != valid {
		t.Fatalf("normalizeSHA256 = %q, want %q", got, valid)
	}

	// 拿不到摘要时必须判为无效：更新器会据此拒绝安装，而不是无校验放行。
	rejected := []string{
		"",
		"   ",
		valid[:63],
		valid + "0",
		strings.Repeat("z", 64),
	}
	for _, candidate := range rejected {
		if _, ok := normalizeSHA256(candidate); ok {
			t.Errorf("normalizeSHA256(%q) accepted an invalid digest", candidate)
		}
	}
}

func TestUpdateDownloadHostAllowed(t *testing.T) {
	allowed := []string{
		"github.com",
		"GitHub.com",
		"objects.githubusercontent.com",
		"githubusercontent.com",
		"api.gitcode.com",
		"gitcode.com",
		"gitcode.com.", // 带根点的完全限定域名
	}
	for _, host := range allowed {
		if !updateDownloadHostAllowed(host) {
			t.Errorf("updateDownloadHostAllowed(%q) = false, want true", host)
		}
	}

	// 后缀拼接类仿冒域名必须被拒绝，否则 evil-gitcode.com 之类会被误放行。
	rejected := []string{
		"",
		"example.com",
		"evilgithubusercontent.com",
		"gitcode.com.evil.net",
		"notgitcode.com",
		"github.com.evil.net",
	}
	for _, host := range rejected {
		if updateDownloadHostAllowed(host) {
			t.Errorf("updateDownloadHostAllowed(%q) = true, want false", host)
		}
	}
}
