package coreapp

import (
	"fmt"
	"runtime"
	"time"

	"github.com/TIANLI0/THRM/internal/coolingbenefit"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/TIANLI0/THRM/internal/version"
)

// GetCoolingBenefit 返回最近一次实测报告与日常统计。
func (a *CoreApp) GetCoolingBenefit() types.CoolingBenefitPayload {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.coolingBenefitPayload(a.configManager.Get())
}

// SaveCoolingBenefitReport 分析并保存一次扫描测试结果，覆盖上一次。
//
// 只保留最近一份而不是历史列表：这类测试的可比性依赖同一个负载，跨次对比本身就
// 不成立，攒一堆只会诱导用户去比较不该比较的东西。
func (a *CoreApp) SaveCoolingBenefitReport(params ipc.SaveCoolingBenefitReportParams) (types.CoolingBenefitPayload, error) {
	if len(params.Steps) < 2 {
		return types.CoolingBenefitPayload{}, fmt.Errorf("散热收益测试至少需要两个转速档位")
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	report := types.CoolingBenefitReport{
		CreatedAt:   time.Now().Unix(),
		DeviceModel: params.DeviceModel,
		CPUModel:    params.CPUModel,
		GPUModel:    params.GPUModel,
		LoadLabel:   params.LoadLabel,
		Steps:       params.Steps,
	}
	// 噪音档案只是可选佐料：有实测就能把"收益拐点"细化成"每分贝换到最多收益"的
	// 甜点转速，没有也不影响报告成立。
	report.Analysis = coolingbenefit.AnalyzeReport(params.Steps, cfg.SmartControl.NoiseProfile)
	cfg.CoolingBenefit.Report = &report

	if err := a.configManager.Update(cfg); err != nil {
		return types.CoolingBenefitPayload{}, err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	a.logInfo("已保存散热收益实测报告: %d 个档位, 判定=%s", len(params.Steps), report.Analysis.Regime)
	return a.coolingBenefitPayload(cfg), nil
}

// ClearCoolingBenefit 清除实测报告和/或日常统计。
func (a *CoreApp) ClearCoolingBenefit(params ipc.ClearCoolingBenefitParams) (types.CoolingBenefitPayload, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if params.Report {
		cfg.CoolingBenefit.Report = nil
	}
	if params.Passive {
		cfg.CoolingBenefit.Passive = types.CoolingPassiveStats{}
	}

	if err := a.configManager.Update(cfg); err != nil {
		return types.CoolingBenefitPayload{}, err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return a.coolingBenefitPayload(cfg), nil
}

// ExportCoolingBenefitText 渲染一份便于语言模型分析的纯文本报告。
//
// 渲染放在核心侧而不是 GUI：分析结论、功耗分桶边界、告警语义都在这里，
// 让两边各写一份格式化逻辑迟早会说出不一致的话。
func (a *CoreApp) ExportCoolingBenefitText() (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if cfg.CoolingBenefit.Report == nil && len(cfg.CoolingBenefit.Passive.Cells) == 0 {
		return "", fmt.Errorf("尚无可导出的散热收益数据")
	}

	return coolingbenefit.FormatTextReport(coolingbenefit.TextReportInput{
		Report:      cfg.CoolingBenefit.Report,
		Passive:     cfg.CoolingBenefit.Passive,
		Comparisons: coolingbenefit.ComparePassive(cfg.CoolingBenefit.Passive),
		// 甜点转速是从噪音档案算出来的，带上档案模型才能复核那个结论。
		NoiseProfile: cfg.SmartControl.NoiseProfile,
		AppVersion:   version.BuildVersion,
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
		GeneratedAt:  time.Now(),
	}), nil
}

func (a *CoreApp) coolingBenefitPayload(cfg types.AppConfig) types.CoolingBenefitPayload {
	return types.CoolingBenefitPayload{
		Report:            cfg.CoolingBenefit.Report,
		Passive:           cfg.CoolingBenefit.Passive,
		PassiveComparison: coolingbenefit.ComparePassive(cfg.CoolingBenefit.Passive),
		PowerBucketBounds: coolingbenefit.PowerBucketBounds,
		MinCellSamples:    coolingbenefit.MinCellSamples,
	}
}
