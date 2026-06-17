package config

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/types"
)

func applyMissingFanFeatureDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	speedDefaults := types.GetDefaultSpeedAvoidanceConfig()
	if rawSpeedAvoidance, ok := rawConfig["speedAvoidance"]; !ok {
		cfg.SpeedAvoidance = speedDefaults
	} else {
		var speedConfig map[string]json.RawMessage
		if err := json.Unmarshal(rawSpeedAvoidance, &speedConfig); err != nil {
			cfg.SpeedAvoidance = speedDefaults
		} else {
			if _, ok := speedConfig["enabled"]; !ok {
				cfg.SpeedAvoidance.Enabled = speedDefaults.Enabled
			}
			if _, ok := speedConfig["minRpm"]; !ok {
				cfg.SpeedAvoidance.MinRPM = speedDefaults.MinRPM
			}
			if _, ok := speedConfig["maxRpm"]; !ok {
				cfg.SpeedAvoidance.MaxRPM = speedDefaults.MaxRPM
			}
			if _, ok := speedConfig["marginRpm"]; !ok {
				cfg.SpeedAvoidance.MarginRPM = speedDefaults.MarginRPM
			}
			if _, ok := speedConfig["emergencyBypassTemp"]; !ok {
				cfg.SpeedAvoidance.EmergencyBypassTemp = speedDefaults.EmergencyBypassTemp
			}
		}
	}
	cfg.SpeedAvoidance = types.NormalizeSpeedAvoidanceConfig(cfg.SpeedAvoidance)

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

	// 旧配置缺失窗口模糊设置时默认 auto(随系统版本)；休眠归零默认开启。
	if _, ok := rawConfig["windowBlur"]; !ok {
		cfg.WindowBlur = types.WindowBlurAuto
	}
	cfg.WindowBlur = types.NormalizeWindowBlur(cfg.WindowBlur)
	// suspendFanOff 默认关闭(零值即 false)，缺失时无需额外处理。
}
