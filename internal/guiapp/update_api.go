package guiapp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

const (
	updateInstallerName    = "THRM-amd64-installer.exe"
	updateDownloadTimeout  = 10 * time.Minute
	updateProgressMinDelta = 2
)

// UpdateProgress 通过 update-download-progress 事件向前端汇报下载进度。
type UpdateProgress struct {
	Percent  int    `json:"percent"`  // 0-100，-1 表示长度未知
	Received int64  `json:"received"` // 已下载字节数
	Total    int64  `json:"total"`    // 总字节数(未知时为 0)
	Stage    string `json:"stage"`    // downloading / installing / done / error
	Message  string `json:"message"`  // 附带信息(错误时为错误文本)
}

func (a *App) emitUpdateProgress(p UpdateProgress) {
	a.emit(eventUpdateProgress, p)
}

// DownloadAndInstallUpdate 下载安装包并以静默(/S)方式全自动安装，期间弹出一个 CMD
// 状态窗口展示更新动态，安装完成后自动重启。用户无需在安装向导中做任何确认（仅保留
// 系统级 UAC 授权）。windowTitle/windowBody/windowRestarting 为状态窗口的本地化文案，
// 由前端按当前界面语言传入；为空时回退到中文默认文案。
func (a *App) DownloadAndInstallUpdate(downloadURL, windowTitle, windowBody, windowRestarting string) error {
	if strings.TrimSpace(windowTitle) == "" {
		windowTitle = "THRM 正在更新"
	}
	if strings.TrimSpace(windowBody) == "" {
		windowBody = "正在自动安装新版本，请勿关闭此窗口"
	}
	if strings.TrimSpace(windowRestarting) == "" {
		windowRestarting = "更新完成，正在重启应用"
	}

	parsed, err := url.Parse(strings.TrimSpace(downloadURL))
	if err != nil || parsed.Scheme != "https" {
		e := fmt.Errorf("无效的下载地址")
		a.emitUpdateProgress(UpdateProgress{Percent: -1, Stage: "error", Message: e.Error()})
		return e
	}
	if !isAllowedUpdateHost(parsed.Hostname()) {
		e := fmt.Errorf("下载地址不在允许的来源内: %s", parsed.Hostname())
		a.emitUpdateProgress(UpdateProgress{Percent: -1, Stage: "error", Message: e.Error()})
		return e
	}

	exePath, exeErr := os.Executable()
	if exeErr != nil {
		guiLogger.Warnf("获取当前可执行文件路径失败，安装后可能无法自动重启: %v", exeErr)
		exePath = ""
	}

	updateDir, err := createUpdateDir()
	if err != nil {
		guiLogger.Errorf("创建更新目录失败: %v", err)
		a.emitUpdateProgress(UpdateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return err
	}

	var installerPath string
	for i, candidate := range updateDownloadCandidates(parsed) {
		if i > 0 {
			guiLogger.Warnf("从官网获取更新失败，改用 GitHub 重试: %v", err)
			a.emitUpdateProgress(UpdateProgress{Percent: 0, Stage: "downloading"})
		}
		installerPath, err = a.downloadUpdateInstaller(candidate, updateDir)
		if err != nil {
			continue
		}
		// 安装包会被提权执行：签名不对的文件既不运行也不留在磁盘上。
		// 校验放在循环内，官网反代给出的文件不可信时还能回退到 GitHub。
		if err = verifyUpdateInstaller(installerPath, exePath); err != nil {
			_ = os.Remove(installerPath)
			continue
		}
		break
	}
	if err != nil {
		guiLogger.Errorf("获取更新安装包失败: %v", err)
		_ = os.RemoveAll(updateDir)
		a.emitUpdateProgress(UpdateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return err
	}

	a.emitUpdateProgress(UpdateProgress{Percent: 100, Stage: "installing", Message: ""})

	if err := launchUpdateInstaller(installerPath, exePath, windowTitle, windowBody, windowRestarting); err != nil {
		guiLogger.Errorf("启动更新安装程序失败: %v", err)
		a.emitUpdateProgress(UpdateProgress{Percent: 100, Stage: "error", Message: err.Error()})
		return err
	}

	// 静默安装已在后台启动；退出当前进程以便安装程序覆盖文件，安装完成后由辅助进程自动重启。
	go func() {
		time.Sleep(800 * time.Millisecond)
		if a.app != nil {
			a.app.Quit()
		} else {
			os.Exit(0)
		}
	}()
	return nil
}

// isAllowedUpdateHost 只允许从官网反代和 GitHub 下载安装包。host 不带端口。
// 后缀匹配必须带前导点，否则 evilgithubusercontent.com 这类域名也会通过。
func isAllowedUpdateHost(host string) bool {
	host = strings.ToLower(host)
	return host == appmeta.WebsiteHost ||
		host == "github.com" || strings.HasSuffix(host, ".github.com") ||
		host == "githubusercontent.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// checkUpdateRedirect 让重定向的每一跳都受同一份来源白名单约束。GitHub Release 的
// 下载地址本身会跳到 *.githubusercontent.com，所以不能禁止重定向，只能逐跳检查。
func checkUpdateRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("重定向次数过多")
	}
	if req.URL.Scheme != "https" || !isAllowedUpdateHost(req.URL.Hostname()) {
		return fmt.Errorf("下载被重定向到不允许的来源: %s://%s", req.URL.Scheme, req.URL.Hostname())
	}
	return nil
}

// updateDownloadCandidates 返回依次尝试的下载地址。官网反代的地址
// （/download/<标签>/<文件> 或 /download/latest/<文件>）后面追加对应的 GitHub 直链，
// 反代不可用时自动回退到 GitHub。
func updateDownloadCandidates(u *url.URL) []string {
	candidates := []string{u.String()}
	if !strings.EqualFold(u.Host, appmeta.WebsiteHost) {
		return candidates
	}
	parts := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	if len(parts) != 3 || parts[0] != "download" || parts[1] == "" || parts[2] == "" {
		return candidates
	}
	if parts[1] == "latest" {
		return append(candidates, appmeta.RepositoryURL+"/releases/latest/download/"+parts[2])
	}
	return append(candidates, appmeta.RepositoryURL+"/releases/download/"+parts[1]+"/"+parts[2])
}

func (a *App) downloadUpdateInstaller(downloadURL, dir string) (string, error) {
	target := filepath.Join(dir, updateInstallerName)

	// 清理可能残留的旧安装包，避免半成品文件被误用。
	_ = os.Remove(target)

	ctx, cancel := context.WithTimeout(context.Background(), updateDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("构造下载请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{CheckRedirect: checkUpdateRedirect}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(target)
	if err != nil {
		return "", fmt.Errorf("创建安装包文件失败: %w", err)
	}
	defer out.Close()

	total := resp.ContentLength
	a.emitUpdateProgress(UpdateProgress{Percent: 0, Total: max(total, 0), Stage: "downloading"})

	buf := make([]byte, 64*1024)
	var received int64
	lastPercent := -100
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return "", fmt.Errorf("写入安装包失败: %w", writeErr)
			}
			received += int64(n)
			percent := -1
			if total > 0 {
				percent = int(received * 100 / total)
			}
			if percent < 0 || percent-lastPercent >= updateProgressMinDelta || percent >= 100 {
				lastPercent = percent
				a.emitUpdateProgress(UpdateProgress{Percent: percent, Received: received, Total: max(total, 0), Stage: "downloading"})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("下载中断: %w", readErr)
		}
	}

	if err := out.Sync(); err != nil {
		return "", fmt.Errorf("刷新安装包到磁盘失败: %w", err)
	}
	// 先关闭再返回：调用方紧接着要校验签名并执行它，不能留着写句柄。
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("关闭安装包文件失败: %w", err)
	}
	return target, nil
}
