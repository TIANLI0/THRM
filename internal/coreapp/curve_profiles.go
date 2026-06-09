package coreapp

import (
	"fmt"
	"strings"

	cfgpkg "github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/curveprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *CoreApp) fanCurveProfilesPayloadFromConfig(cfg types.AppConfig) types.FanCurveProfilesPayload {
	return types.FanCurveProfilesPayload{
		Profiles: curveprofiles.CloneProfiles(cfg.FanCurveProfiles),
		ActiveID: cfg.ActiveFanCurveProfileID,
	}
}

func (a *CoreApp) applyCurveProfilesConfig(cfg types.AppConfig) error {
	syncSmartControlOffsetsForActiveProfile(&cfg)
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	storeSmartControlOffsetsForActiveProfile(&cfg)
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return nil
}

func (a *CoreApp) GetFanCurveProfiles() types.FanCurveProfilesPayload {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if curveprofiles.NormalizeConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存温控曲线方案默认配置失败: %v", err)
		}
	}
	return a.fanCurveProfilesPayloadFromConfig(cfg)
}

func (a *CoreApp) SetActiveFanCurveProfile(profileID string) (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	curveprofiles.NormalizeConfig(&cfg)
	storeSmartControlOffsetsForActiveProfile(&cfg)

	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		return types.FanCurveProfile{}, fmt.Errorf("未找到温控曲线方案")
	}

	cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[idx].ID
	cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[idx].Curve)
	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}
	return types.FanCurveProfile{
		ID:    cfg.FanCurveProfiles[idx].ID,
		Name:  cfg.FanCurveProfiles[idx].Name,
		Curve: curveprofiles.CloneCurve(cfg.FanCurveProfiles[idx].Curve),
	}, nil
}

func (a *CoreApp) CycleFanCurveProfile() (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	curveprofiles.NormalizeConfig(&cfg)
	storeSmartControlOffsetsForActiveProfile(&cfg)

	if len(cfg.FanCurveProfiles) == 0 {
		return types.FanCurveProfile{}, fmt.Errorf("暂无可用温控曲线方案")
	}

	idx := max(curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID), 0)
	nextIdx := (idx + 1) % len(cfg.FanCurveProfiles)
	cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[nextIdx].ID
	cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[nextIdx].Curve)

	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}

	return types.FanCurveProfile{
		ID:    cfg.FanCurveProfiles[nextIdx].ID,
		Name:  cfg.FanCurveProfiles[nextIdx].Name,
		Curve: curveprofiles.CloneCurve(cfg.FanCurveProfiles[nextIdx].Curve),
	}, nil
}

func (a *CoreApp) SaveFanCurveProfile(params ipc.SaveFanCurveProfileParams) (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	curveprofiles.NormalizeConfig(&cfg)
	storeSmartControlOffsetsForActiveProfile(&cfg)

	curve := curveprofiles.CloneCurve(params.Curve)
	if err := cfgpkg.ValidateFanCurve(curve); err != nil {
		return types.FanCurveProfile{}, err
	}

	profileName := curveprofiles.NormalizeProfileName(params.Name, "新曲线")
	profileID := strings.TrimSpace(params.ID)
	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		profileID = curveprofiles.GenerateID()
		cfg.FanCurveProfiles = append(cfg.FanCurveProfiles, types.FanCurveProfile{
			ID:    profileID,
			Name:  profileName,
			Curve: curve,
		})
		idx = len(cfg.FanCurveProfiles) - 1
	} else {
		cfg.FanCurveProfiles[idx].Name = profileName
		cfg.FanCurveProfiles[idx].Curve = curve
	}

	if params.SetActive || cfg.ActiveFanCurveProfileID == cfg.FanCurveProfiles[idx].ID {
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[idx].ID
		cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[idx].Curve)
	}

	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}

	updated := cfg.FanCurveProfiles[idx]
	return types.FanCurveProfile{ID: updated.ID, Name: updated.Name, Curve: curveprofiles.CloneCurve(updated.Curve)}, nil
}

func (a *CoreApp) DeleteFanCurveProfile(profileID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	curveprofiles.NormalizeConfig(&cfg)

	if len(cfg.FanCurveProfiles) <= 1 {
		return fmt.Errorf("至少保留一个温控曲线方案")
	}

	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		return fmt.Errorf("未找到温控曲线方案")
	}

	cfg.FanCurveProfiles = append(cfg.FanCurveProfiles[:idx], cfg.FanCurveProfiles[idx+1:]...)
	if len(cfg.FanCurveProfiles) == 0 {
		return fmt.Errorf("至少保留一个温控曲线方案")
	}

	if cfg.ActiveFanCurveProfileID == profileID {
		nextIdx := idx
		if nextIdx >= len(cfg.FanCurveProfiles) {
			nextIdx = len(cfg.FanCurveProfiles) - 1
		}
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[nextIdx].ID
		cfg.FanCurve = curveprofiles.CloneCurve(cfg.FanCurveProfiles[nextIdx].Curve)
	}

	return a.applyCurveProfilesConfig(cfg)
}

func (a *CoreApp) ExportFanCurveProfiles() (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	curveprofiles.NormalizeConfig(&cfg)
	if idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}

	return curveprofiles.Export(cfg.ActiveFanCurveProfileID, cfg.FanCurveProfiles)
}

func (a *CoreApp) ImportFanCurveProfiles(code string) error {
	profiles, activeID, err := curveprofiles.Import(code)
	if err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.FanCurveProfiles = curveprofiles.CloneProfiles(profiles)
	cfg.ActiveFanCurveProfileID = activeID
	if curveprofiles.NormalizeConfig(&cfg) {
		// normalized in place
	}

	return a.applyCurveProfilesConfig(cfg)
}
