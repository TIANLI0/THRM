package coreapp

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/fwflash"
	"github.com/TIANLI0/THRM/internal/fwupdate"
	"github.com/TIANLI0/THRM/internal/ipc"
)

// FirmwareUpdateStatus 是固件更新面板所需的设备/驱动状态。
type FirmwareUpdateStatus struct {
	Supported        bool   `json:"supported"`        // 当前平台是否支持（仅 Windows）
	DeviceCode       string `json:"deviceCode"`       // bs2pro/bs3pro…
	ModelName        string `json:"modelName"`        // 展示名
	NormalConnected  bool   `json:"normalConnected"`  // 正常固件模式下已连接
	DevicePresentISP bool   `json:"devicePresentIsp"` // 处于下载模式（ISP）
	DriverInstalled  bool   `json:"driverInstalled"`  // 已绑定 WinUSB，可直接刷写
	CurrentVersion   string `json:"currentVersion"`   // 当前设备固件版本（如 0.0.2.4）
	ThrmVersion      int    `json:"thrmVersion"`      // THRM 定制固件版本（0=原厂）
}

// firmwareFlashEvent 是 EventFirmwareUpdate 的载荷。
type firmwareFlashEvent struct {
	Kind    string `json:"kind"`    // log | progress | done
	Phase   string `json:"phase"`   // write | verify
	Done    int    `json:"done"`    // progress: 已处理字节
	Total   int    `json:"total"`   // progress: 总字节
	Message string `json:"message"` // log/done 文本
	Success bool   `json:"success"` // done: 是否成功
}

var flashMu sync.Mutex // 串行化刷写，避免并发

var errFirmwareUpdateDebugOnly = errors.New("固件更新仅在调试模式下可用，请先开启调试模式")

func (a *CoreApp) requireFirmwareUpdateDebugMode() error {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if !a.debugMode {
		return errFirmwareUpdateDebugOnly
	}
	return nil
}

// GetFirmwareUpdateStatus 返回固件更新面板状态。
func (a *CoreApp) GetFirmwareUpdateStatus() FirmwareUpdateStatus {
	if a.requireFirmwareUpdateDebugMode() != nil {
		return FirmwareUpdateStatus{}
	}
	st := FirmwareUpdateStatus{
		Supported:       runtime.GOOS == "windows",
		NormalConnected: a.deviceManager.IsConnected(),
		ModelName:       a.deviceManager.GetModelName(),
		DeviceCode:      fwupdate.DeviceCode(a.deviceManager.GetProductID()),
	}
	// 正常连接时拿不到 ProductID（0），用型号名兜底映射。
	if st.DeviceCode == "" {
		st.DeviceCode = deviceCodeFromModel(st.ModelName)
	}
	drv := fwflash.QueryDriverStatus()
	st.DevicePresentISP = drv.DevicePresent
	st.DriverInstalled = drv.WinUSBBound

	a.mutex.RLock()
	if a.deviceSettings != nil {
		st.CurrentVersion = a.deviceSettings.FirmwareVersion
		st.ThrmVersion = a.deviceSettings.ThrmFirmwareVersion
	}
	a.mutex.RUnlock()
	return st
}

func deviceCodeFromModel(model string) string {
	switch {
	case containsFold(model, "BS3PRO"):
		return "bs3pro"
	case containsFold(model, "BS2PRO"):
		return "bs2pro"
	case containsFold(model, "BS3"):
		return "bs3"
	case containsFold(model, "BS2"):
		return "bs2"
	}
	return ""
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && indexFold(s, sub) >= 0
}

func indexFold(s, sub string) int {
	ls, lsub := len(s), len(sub)
	for i := 0; i+lsub <= ls; i++ {
		ok := true
		for j := 0; j < lsub; j++ {
			c1, c2 := s[i+j], sub[j]
			if 'a' <= c1 && c1 <= 'z' {
				c1 -= 'a' - 'A'
			}
			if 'a' <= c2 && c2 <= 'z' {
				c2 -= 'a' - 'A'
			}
			if c1 != c2 {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// ListFirmwareUpdates 返回可刷固件清单。按当前固件类型决定主来源：
// 当前跑 THRM 自定义固件（ThrmVersion>0）→ 优先查 GitHub 上的自定义固件；
// 当前跑官方固件 → 优先查官方。两类都会附上以便切换，匹配的那类排在最前（默认选中）。
func (a *CoreApp) ListFirmwareUpdates() ([]fwupdate.FirmwareInfo, error) {
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return nil, err
	}
	st := a.GetFirmwareUpdateStatus()
	code := st.DeviceCode
	if code == "" {
		return nil, fmt.Errorf("无法识别设备型号，请先连接设备")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	official := func() []fwupdate.FirmwareInfo {
		if off, err := fwupdate.FetchOfficial(ctx, code); err == nil && off != nil {
			return []fwupdate.FirmwareInfo{*off}
		}
		return nil
	}
	custom := func() []fwupdate.FirmwareInfo {
		list, _ := fwupdate.FetchCustom(ctx, "", code)
		return list
	}

	var list []fwupdate.FirmwareInfo
	if st.ThrmVersion > 0 {
		list = append(list, custom()...)
		list = append(list, official()...)
	} else {
		list = append(list, official()...)
		list = append(list, custom()...)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("暂无可用固件")
	}
	return list, nil
}

// InstallFirmwareDriver 为处于下载模式的设备绑定 WinUSB 驱动（需管理员，Core 已提权）。
func (a *CoreApp) InstallFirmwareDriver() error {
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return err
	}
	return fwflash.InstallDriver()
}

// UninstallFirmwareDriver 移除 THRM 装的 WinUSB 绑定。
func (a *CoreApp) UninstallFirmwareDriver() error {
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return err
	}
	return fwflash.UninstallDriver()
}

// flashLogger 把 fwflash 的日志/进度转成 EventFirmwareUpdate 广播给前端。
type flashLogger struct{ a *CoreApp }

func (l flashLogger) Infof(format string, args ...any) {
	l.a.emitFlash(firmwareFlashEvent{Kind: "log", Message: fmt.Sprintf(format, args...)})
}
func (l flashLogger) Progress(phase string, done, total int) {
	l.a.emitFlash(firmwareFlashEvent{Kind: "progress", Phase: phase, Done: done, Total: total})
}

func (a *CoreApp) emitFlash(ev firmwareFlashEvent) {
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFirmwareUpdate, ev)
	}
}

// FlashFirmware 异步执行刷写：加载固件→（必要时装驱动）→擦/写/校验/复位。
// 立即返回；进度、日志、完成通过 EventFirmwareUpdate 下发。
func (a *CoreApp) FlashFirmware(params ipc.FlashFirmwareParams) error {
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("固件刷写目前仅支持 Windows")
	}
	a.safeGo("firmware-flash", func() {
		if !flashMu.TryLock() {
			a.emitFlash(firmwareFlashEvent{Kind: "done", Message: "已有刷写任务在进行中", Success: false})
			return
		}
		defer flashMu.Unlock()
		if err := a.runFlash(params); err != nil {
			a.logError("固件刷写失败: %v", err)
			a.emitFlash(firmwareFlashEvent{Kind: "done", Message: err.Error(), Success: false})
			return
		}
		a.emitFlash(firmwareFlashEvent{Kind: "done", Message: "刷写完成，设备已复位", Success: true})
	})
	return nil
}

func (a *CoreApp) runFlash(params ipc.FlashFirmwareParams) error {
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return err
	}
	log := flashLogger{a}

	// 1) 准备固件字节
	var raw []byte
	var err error
	if params.Source == fwupdate.SourceLocal {
		log.Infof("读取本地固件: %s", params.LocalPath)
		raw, err = fwflash.LoadFirmware(params.LocalPath)
	} else {
		log.Infof("下载固件: %s", params.Version)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var data []byte
		data, err = fwupdate.Download(ctx, params.URL)
		if err == nil {
			raw, err = fwflash.ParseFirmwareBytes(params.URL, data)
		}
	}
	if err != nil {
		return err
	}
	log.Infof("固件大小: %d 字节", len(raw))
	// Debug mode may have been disabled while downloading. Reject queued work
	// before touching the driver/device. An already-started flash is allowed
	// to finish; interrupting an erase/write would leave the device incomplete.
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return err
	}

	// 2) 确认设备在下载模式
	drv := fwflash.QueryDriverStatus()
	if !drv.DevicePresent {
		return fmt.Errorf("未检测到下载模式设备：请按住按键后插入 USB，再重试")
	}

	// 3) 必要时安装 WinUSB 驱动
	if !drv.WinUSBBound {
		log.Infof("安装 WinUSB 驱动……")
		if err := a.InstallFirmwareDriver(); err != nil {
			return err
		}
		// 驱动绑定后设备会重新枚举，稍等再打开
		time.Sleep(2 * time.Second)
	}

	// 4) 打开并刷写
	if err := a.requireFirmwareUpdateDebugMode(); err != nil {
		return err
	}
	t, err := fwflash.OpenUSB()
	if err != nil {
		return err
	}
	f, err := fwflash.Open(t, log)
	if err != nil {
		t.Close()
		return err
	}
	defer f.Close()
	log.Infof("%s", f.Info())
	return f.Flash(raw, true, true)
}
