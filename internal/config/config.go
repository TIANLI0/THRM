// Package config 提供配置管理功能
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

// Manager 配置管理器
//
// 注意：内部所有对 config 字段的读写均通过 mu 保护，避免并发场景下的数据竞争。
// 公共方法（Get/Set/Update/Save/Load）是并发安全的；不要在持有 mu 的情况下调用
// 这些公共方法（会自死锁）；内部需要无锁版本时使用以 Locked 结尾的私有方法。
type Manager struct {
	mu         sync.RWMutex
	config     types.AppConfig
	revision   uint64
	installDir string
	logger     types.Logger
}

// NewManager 创建新的配置管理器
func NewManager(installDir string, logger types.Logger) *Manager {
	return &Manager{
		installDir: installDir,
		logger:     logger,
	}
}

// Load 加载配置
func (m *Manager) Load(isAutoStart bool) types.AppConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 优先尝试从默认目录加载配置
	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")
	legacyConfigPath := filepath.Join(m.GetLegacyConfigDir(), "config.json")

	installConfigPath := filepath.Join(m.installDir, "config", "config.json")

	m.logInfo("尝试从默认目录加载配置: %s", defaultConfigPath)

	// 先尝试从默认目录加载
	if m.tryLoadFromPathLocked(defaultConfigPath) {
		m.config.ConfigPath = defaultConfigPath
		m.logInfo("从默认目录加载配置成功: %s", defaultConfigPath)
		m.bumpRevisionLocked()
		return m.config
	}

	m.logInfo("从默认目录加载配置失败，尝试从旧目录加载: %s", legacyConfigPath)

	if m.tryLoadFromPathLocked(legacyConfigPath) {
		m.config.ConfigPath = defaultConfigPath
		m.logInfo("从旧目录加载配置成功，将迁移到新目录: %s", legacyConfigPath)
		if err := m.saveLocked(); err != nil {
			m.logError("迁移旧目录配置失败: %v", err)
		}
		m.bumpRevisionLocked()
		return m.config
	}

	m.logInfo("从默认目录加载配置失败，尝试从安装目录加载: %s", installConfigPath)

	// 默认目录失败，尝试从安装目录加载
	if m.tryLoadFromPathLocked(installConfigPath) {
		m.config.ConfigPath = installConfigPath
		m.logInfo("从安装目录加载配置成功: %s", installConfigPath)
		m.bumpRevisionLocked()
		return m.config
	}

	m.logError("所有配置目录加载失败，使用默认配置")

	m.config = types.GetDefaultConfig(isAutoStart)
	m.config.ConfigPath = defaultConfigPath
	if err := m.saveLocked(); err != nil {
		m.logError("保存默认配置失败: %v", err)
	}
	m.bumpRevisionLocked()

	return m.config
}

// tryLoadFromPathLocked 尝试从指定路径加载配置（调用方需持有 m.mu）
func (m *Manager) tryLoadFromPathLocked(configPath string) bool {
	if _, err := os.Stat(configPath); err != nil {
		m.logDebug("配置文件不存在: %s", configPath)
		return false
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		m.logError("读取配置文件失败 %s: %v", configPath, err)
		return false
	}

	var rawConfig map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		m.logError("解析配置文件元数据失败 %s: %v", configPath, err)
		return false
	}

	var config types.AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		m.logError("解析配置文件失败 %s: %v", configPath, err)
		return false
	}

	applyMissingHotkeyDefaults(&config, rawConfig)
	applyMissingSmartControlDefaults(&config, rawConfig)
	applyMissingLegionFnQDefaults(&config, rawConfig)
	applyMissingThemeDefaults(&config, rawConfig)
	applyMissingTemperatureDefaults(&config, rawConfig)
	applyMissingFanFeatureDefaults(&config, rawConfig)

	m.config = config
	return true
}

func applyMissingHotkeyDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultConfig(false)
	if _, ok := rawConfig["manualGearToggleHotkey"]; !ok {
		cfg.ManualGearToggleHotkey = defaults.ManualGearToggleHotkey
	}
	if _, ok := rawConfig["autoControlToggleHotkey"]; !ok {
		cfg.AutoControlToggleHotkey = defaults.AutoControlToggleHotkey
	}
	if _, ok := rawConfig["curveProfileToggleHotkey"]; !ok {
		cfg.CurveProfileToggleHotkey = defaults.CurveProfileToggleHotkey
	}
}

func applyMissingSmartControlDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultSmartControlConfig(cfg.FanCurve)
	rawSmartControl, ok := rawConfig["smartControl"]
	if !ok {
		cfg.SmartControl.FilterTransientSpike = defaults.FilterTransientSpike
		cfg.SmartControl.LearningBias = defaults.LearningBias
		return
	}

	var smartControlConfig map[string]json.RawMessage
	if err := json.Unmarshal(rawSmartControl, &smartControlConfig); err != nil {
		return
	}

	if _, ok := smartControlConfig["filterTransientSpike"]; !ok {
		cfg.SmartControl.FilterTransientSpike = defaults.FilterTransientSpike
	}
	if _, ok := smartControlConfig["learningBias"]; !ok {
		cfg.SmartControl.LearningBias = defaults.LearningBias
	}
}

func applyMissingLegionFnQDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}
	if _, ok := rawConfig["legionFnQ"]; !ok {
		cfg.LegionFnQ = types.GetDefaultLegionFnQConfig()
		return
	}
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
}

func applyMissingThemeDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaultThemeMode := types.ThemeModeSystem
	if _, ok := rawConfig["themeMode"]; !ok {
		cfg.ThemeMode = defaultThemeMode
		return
	}

	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
}

func applyMissingTemperatureDefaults(cfg *types.AppConfig, rawConfig map[string]json.RawMessage) {
	if cfg == nil {
		return
	}

	defaults := types.GetDefaultTemperatureSelection()
	if _, ok := rawConfig["tempSource"]; !ok {
		cfg.TempSource = defaults.TempSource
	}
	if _, ok := rawConfig["gpuDevice"]; !ok {
		cfg.GpuDevice = defaults.GpuDevice
	}
	if _, ok := rawConfig["cpuSensor"]; !ok {
		cfg.CpuSensor = defaults.CpuSensor
	}
	if _, ok := rawConfig["gpuSensor"]; !ok {
		cfg.GpuSensor = defaults.GpuSensor
	}
	cfg.TempSource = types.NormalizeTempSource(cfg.TempSource)
	cfg.GpuDevice = types.NormalizeDeviceSelection(cfg.GpuDevice)
	cfg.CpuSensor = types.NormalizeSensorSelection(cfg.CpuSensor)
	cfg.CpuSensors = types.NormalizeSensorSelections(cfg.CpuSensors)
	cfg.GpuSensor = types.NormalizeSensorSelection(cfg.GpuSensor)
}

// Save 保存配置（线程安全）
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

// saveLocked 保存配置（调用方需持有 m.mu 写锁）
func (m *Manager) saveLocked() error {
	// 首先尝试保存到默认目录
	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")

	m.logDebug("尝试保存配置到默认目录: %s", defaultConfigPath)

	// 确保默认配置目录存在
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		m.logError("创建默认配置目录失败: %v", err)
	} else {
		data, err := json.MarshalIndent(m.config, "", "  ")
		if err != nil {
			m.logError("序列化配置失败: %v", err)
		} else {
			if err := os.WriteFile(defaultConfigPath, data, 0644); err != nil {
				m.logError("保存配置到默认目录失败: %v", err)
			} else {
				m.config.ConfigPath = defaultConfigPath
				m.logInfo("配置保存到默认目录成功: %s", defaultConfigPath)
				return nil
			}
		}
	}

	installConfigDir := filepath.Join(m.installDir, "config")
	installConfigPath := filepath.Join(installConfigDir, "config.json")

	m.logInfo("保存到默认目录失败，尝试保存到安装目录: %s", installConfigPath)

	if err := os.MkdirAll(installConfigDir, 0755); err != nil {
		m.logError("创建安装配置目录失败: %v", err)
		return err
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		m.logError("序列化配置失败: %v", err)
		return err
	}

	if err := os.WriteFile(installConfigPath, data, 0644); err != nil {
		m.logError("保存配置到安装目录失败: %v", err)
		return err
	}

	m.config.ConfigPath = installConfigPath
	m.logInfo("配置保存到安装目录成功: %s", installConfigPath)
	return nil
}

// GetDefaultConfigDir 获取默认配置目录
func (m *Manager) GetDefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.logError("获取用户主目录失败: %v", err)
		return filepath.Join(m.installDir, "config")
	}
	return appmeta.UserConfigDir(homeDir)
}

// GetLegacyConfigDir 获取旧版本配置目录
func (m *Manager) GetLegacyConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(m.installDir, "config")
	}
	return appmeta.LegacyUserConfigDir(homeDir)
}

// Get 获取当前配置（线程安全，返回拷贝）
func (m *Manager) Get() types.AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *Manager) GetWithRevision() (types.AppConfig, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config, m.revision
}

// Set 设置配置（线程安全）
func (m *Manager) Set(config types.AppConfig) {
	m.mu.Lock()
	m.config = config
	m.bumpRevisionLocked()
	m.mu.Unlock()
}

// Update 更新配置并保存（线程安全，原子操作）
func (m *Manager) Update(config types.AppConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	m.bumpRevisionLocked()
	return m.saveLocked()
}

func (m *Manager) bumpRevisionLocked() {
	m.revision++
}

// 日志辅助方法
func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		m.logger.Error(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}

// GetConfigDir 获取配置目录（保持向后兼容）
func (m *Manager) GetConfigDir() string {
	return m.GetDefaultConfigDir()
}

// GetInstallDir 获取安装目录
func GetInstallDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// GetCurrentWorkingDir 获取当前工作目录
func GetCurrentWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

// ValidateFanCurve 验证风扇曲线是否有效
func ValidateFanCurve(curve []types.FanCurvePoint) error {
	if len(curve) < 2 {
		return fmt.Errorf("风扇曲线至少需要2个点")
	}

	for i, point := range curve {
		if point.Temperature > types.FanCurveMaxTemperature {
			return fmt.Errorf("风扇曲线第%d个点温度超出范围(最高%d°C)", i+1, types.FanCurveMaxTemperature)
		}
		if point.RPM < 0 || point.RPM > 4000 {
			return fmt.Errorf("风扇曲线第%d个点RPM超出范围(0-4000)", i+1)
		}
	}

	for i := 1; i < len(curve); i++ {
		if curve[i].Temperature <= curve[i-1].Temperature {
			return fmt.Errorf("风扇曲线温度点必须递增")
		}
		if curve[i].RPM < curve[i-1].RPM {
			return fmt.Errorf("风扇曲线转速点必须从左到右非递减")
		}
	}

	return nil
}
