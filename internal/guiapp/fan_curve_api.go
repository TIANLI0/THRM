package guiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SetFanCurve 设置风扇曲线
func (a *App) SetFanCurve(curve []FanCurvePoint) error {
	resp, err := a.sendRequest(ipc.ReqSetFanCurve, curve)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// ResetLearnedOffsets 清空学习到的曲线偏移
func (a *App) ResetLearnedOffsets() error {
	resp, err := a.sendRequest(ipc.ReqResetLearnedOffsets, nil)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetFanCurve 获取风扇曲线
func (a *App) GetFanCurve() []FanCurvePoint {
	resp, err := a.sendRequest(ipc.ReqGetFanCurve, nil)
	if err != nil {
		return types.GetDefaultFanCurve()
	}
	if !resp.Success {
		return types.GetDefaultFanCurve()
	}
	var curve []FanCurvePoint
	json.Unmarshal(resp.Data, &curve)
	return curve
}

// GetFanCurveProfiles 获取曲线方案列表
func (a *App) GetFanCurveProfiles() FanCurveProfilesPayload {
	resp, err := a.sendRequest(ipc.ReqGetFanCurveProfiles, nil)
	if err != nil || !resp.Success {
		cfg := a.GetConfig()
		return types.FanCurveProfilesPayload{
			Profiles: cfg.FanCurveProfiles,
			ActiveID: cfg.ActiveFanCurveProfileID,
		}
	}
	var payload FanCurveProfilesPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

// SetActiveFanCurveProfile 设置当前激活曲线方案
func (a *App) SetActiveFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqSetActiveFanCurveProfile, ipc.SetActiveFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SaveFanCurveProfile 保存曲线方案
func (a *App) SaveFanCurveProfile(profileID, name string, curve []FanCurvePoint, setActive bool) (FanCurveProfile, error) {
	resp, err := a.sendRequest(ipc.ReqSaveFanCurveProfile, ipc.SaveFanCurveProfileParams{
		ID:        profileID,
		Name:      name,
		Curve:     curve,
		SetActive: setActive,
	})
	if err != nil {
		return FanCurveProfile{}, err
	}
	if !resp.Success {
		return FanCurveProfile{}, fmt.Errorf("%s", resp.Error)
	}
	var profile FanCurveProfile
	json.Unmarshal(resp.Data, &profile)
	return profile, nil
}

// DeleteFanCurveProfile 删除曲线方案
func (a *App) DeleteFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqDeleteFanCurveProfile, ipc.DeleteFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// ExportFanCurveProfiles 导出曲线方案
func (a *App) ExportFanCurveProfiles() (string, error) {
	resp, err := a.sendRequest(ipc.ReqExportFanCurveProfiles, nil)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var code string
	json.Unmarshal(resp.Data, &code)
	return code, nil
}

// ImportFanCurveProfiles 导入曲线方案
func (a *App) ImportFanCurveProfiles(code string) error {
	resp, err := a.sendRequest(ipc.ReqImportFanCurveProfiles, ipc.ImportFanCurveProfilesParams{Code: code})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

/* ── 散热收益 ── */

// GetCoolingBenefit 获取最近一次散热收益实测报告与日常统计。
func (a *App) GetCoolingBenefit() types.CoolingBenefitPayload {
	resp, err := a.sendRequest(ipc.ReqGetCoolingBenefit, nil)
	if err != nil || !resp.Success {
		return types.CoolingBenefitPayload{}
	}
	var payload types.CoolingBenefitPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

// SaveCoolingBenefitReport 提交一次扫描测试的原始档位数据，核心侧分析后保存。
func (a *App) SaveCoolingBenefitReport(params ipc.SaveCoolingBenefitReportParams) (types.CoolingBenefitPayload, error) {
	resp, err := a.sendRequest(ipc.ReqSaveCoolingBenefitReport, params)
	if err != nil {
		return types.CoolingBenefitPayload{}, err
	}
	if !resp.Success {
		return types.CoolingBenefitPayload{}, fmt.Errorf("%s", resp.Error)
	}
	var payload types.CoolingBenefitPayload
	json.Unmarshal(resp.Data, &payload)
	return payload, nil
}

// ClearCoolingBenefit 清除已保存的实测报告。
func (a *App) ClearCoolingBenefit() (types.CoolingBenefitPayload, error) {
	resp, err := a.sendRequest(ipc.ReqClearCoolingBenefit, nil)
	if err != nil {
		return types.CoolingBenefitPayload{}, err
	}
	if !resp.Success {
		return types.CoolingBenefitPayload{}, fmt.Errorf("%s", resp.Error)
	}
	var payload types.CoolingBenefitPayload
	json.Unmarshal(resp.Data, &payload)
	return payload, nil
}

// SetExtendedSensors 临时开启/关闭扩展温度传感器（内存 / 硬盘 / 主板 / EC / 电源）。
//
// 这是会话级的临时开关，不写进配置：这些读数不参与控温，只有散热收益测试需要，
// 而为它们打开的 SMART/SPD/Super I/O 通道会一直消耗后台资源。核心侧还会在 GUI
// 断开或超时后自动关闭，所以即便这里漏发关闭指令也不会一直开着。
func (a *App) SetExtendedSensors(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetExtendedSensors, enabled)
	if err != nil || !resp.Success {
		return false
	}
	var result map[string]bool
	json.Unmarshal(resp.Data, &result)
	return result["enabled"]
}

// ExportCoolingBenefitReport 把散热收益测试结果导出成纯文本，返回保存路径。
// 用户取消保存对话框时返回空路径且不报错。
func (a *App) ExportCoolingBenefitReport() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}

	resp, err := a.sendRequest(ipc.ReqExportCoolingBenefitText, nil)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var text string
	if err := json.Unmarshal(resp.Data, &text); err != nil {
		return "", err
	}

	name := fmt.Sprintf("THRM-cooling-benefit-%s.txt", time.Now().Format("20060102-150405"))
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export cooling benefit report",
		DefaultFilename: name,
		Filters:         []wailsruntime.FileFilter{{DisplayName: "Text file", Pattern: "*.txt"}},
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return "", err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
