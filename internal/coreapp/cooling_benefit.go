package coreapp

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/coolingbenefit"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
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

func (a *CoreApp) coolingBenefitPayload(cfg types.AppConfig) types.CoolingBenefitPayload {
	return types.CoolingBenefitPayload{
		Report:            cfg.CoolingBenefit.Report,
		Passive:           cfg.CoolingBenefit.Passive,
		PassiveComparison: coolingbenefit.ComparePassive(cfg.CoolingBenefit.Passive),
		PowerBucketBounds: coolingbenefit.PowerBucketBounds,
		MinCellSamples:    coolingbenefit.MinCellSamples,
	}
}
