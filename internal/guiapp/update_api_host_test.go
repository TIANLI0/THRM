package guiapp

import "testing"

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
