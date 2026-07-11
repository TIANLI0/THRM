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

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateInstallerName    = "THRM-amd64-installer.exe"
	updateProgressEvent    = "update-download-progress"
	updateDownloadTimeout  = 10 * time.Minute
	updateProgressMinDelta = 2
)

// updateProgress 通过事件向前端汇报下载进度。
type updateProgress struct {
	Percent  int    `json:"percent"`  // 0-100，-1 表示长度未知
	Received int64  `json:"received"` // 已下载字节数
	Total    int64  `json:"total"`    // 总字节数(未知时为 0)
	Stage    string `json:"stage"`    // downloading / installing / done / error
	Message  string `json:"message"`  // 附带信息(错误时为错误文本)
}

func (a *App) emitUpdateProgress(p updateProgress) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, updateProgressEvent, p)
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
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: e.Error()})
		return e
	}
	host := strings.ToLower(parsed.Host)
	if host != "github.com" && !strings.HasSuffix(host, ".github.com") &&
		!strings.HasSuffix(host, "githubusercontent.com") && host != "objects.githubusercontent.com" {
		e := fmt.Errorf("下载地址不在允许的来源内: %s", parsed.Host)
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: e.Error()})
		return e
	}

	installerPath, err := a.downloadUpdateInstaller(parsed.String())
	if err != nil {
		guiLogger.Errorf("下载更新安装包失败: %v", err)
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return err
	}

	a.emitUpdateProgress(updateProgress{Percent: 100, Stage: "installing", Message: ""})

	exePath, exeErr := os.Executable()
	if exeErr != nil {
		guiLogger.Warnf("获取当前可执行文件路径失败，安装后可能无法自动重启: %v", exeErr)
		exePath = ""
	}

	if err := launchUpdateInstaller(installerPath, exePath, windowTitle, windowBody, windowRestarting); err != nil {
		guiLogger.Errorf("启动更新安装程序失败: %v", err)
		a.emitUpdateProgress(updateProgress{Percent: 100, Stage: "error", Message: err.Error()})
		return err
	}

	// 静默安装已在后台启动；退出当前进程以便安装程序覆盖文件，安装完成后由辅助进程自动重启。
	go func() {
		time.Sleep(800 * time.Millisecond)
		if a.ctx != nil {
			runtime.Quit(a.ctx)
		} else {
			os.Exit(0)
		}
	}()
	return nil
}

func (a *App) downloadUpdateInstaller(downloadURL string) (string, error) {
	dir := filepath.Join(os.TempDir(), "THRM-update")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
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

	resp, err := http.DefaultClient.Do(req)
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
	a.emitUpdateProgress(updateProgress{Percent: 0, Total: max64(total, 0), Stage: "downloading"})

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
				a.emitUpdateProgress(updateProgress{Percent: percent, Received: received, Total: max64(total, 0), Stage: "downloading"})
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
	return target, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
