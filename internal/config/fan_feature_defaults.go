package config

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/types"
)

func applyMissingFanFeatureDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	scheduleDefaults := types.GetDefaultTimeCurveScheduleConfig()
	if rawTimeCurveSchedule, ok := rawConfig["timeCurveSchedule"]; !ok {
		cfg.TimeCurveSchedule = scheduleDefaults
	} else {
		var scheduleConfig map[string]json.RawMessage
		if err := json.Unmarshal(rawTimeCurveSchedule, &scheduleConfig); err != nil {
			cfg.TimeCurveSchedule = scheduleDefaults
		} else if _, ok := scheduleConfig["rules"]; !ok {
			cfg.TimeCurveSchedule.Rules = scheduleDefaults.Rules
		}
	}
	cfg.TimeCurveSchedule = types.NormalizeTimeCurveScheduleConfig(cfg.TimeCurveSchedule, cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)

	// 旧配置缺失窗口材质设置时默认 auto(随系统版本)。
	if _, ok := rawConfig["windowBlur"]; !ok {
		cfg.WindowBlur = types.WindowBlurAuto
	}
	cfg.WindowBlur = types.NormalizeWindowBlur(cfg.WindowBlur)
}
