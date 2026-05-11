package plugins

import (
	"context"
	"fmt"
	"sync"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// Manager owns registered plugins and coordinates their lifecycle.
type Manager struct {
	logger  types.Logger
	plugins []Plugin

	mutex  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{logger: logger}
}

func (m *Manager) Register(plugin Plugin) {
	if plugin == nil {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.plugins = append(m.plugins, plugin)
}

func (m *Manager) StartAll(parent context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.ctx != nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	m.ctx, m.cancel = context.WithCancel(parent)

	var firstErr error
	for _, plugin := range m.plugins {
		if err := plugin.Start(m.ctx); err != nil {
			m.logError("plugin start failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", plugin.ID(), err)
			}
			continue
		}
		m.logInfo("plugin started: %s (%s)", plugin.Name(), plugin.ID())
	}

	return firstErr
}

func (m *Manager) Start(pluginID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.ctx == nil {
		m.ctx, m.cancel = context.WithCancel(context.Background())
	}

	plugin := m.findLocked(pluginID)
	if plugin == nil {
		return fmt.Errorf("plugin not registered: %s", pluginID)
	}
	if err := plugin.Start(m.ctx); err != nil {
		m.logError("plugin start failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
		return err
	}
	m.logInfo("plugin started: %s (%s)", plugin.Name(), plugin.ID())
	return nil
}

func (m *Manager) Stop(pluginID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	plugin := m.findLocked(pluginID)
	if plugin == nil {
		return fmt.Errorf("plugin not registered: %s", pluginID)
	}
	if err := plugin.Stop(); err != nil {
		m.logError("plugin stop failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
		return err
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	for _, plugin := range m.plugins {
		if err := plugin.Stop(); err != nil {
			m.logError("plugin stop failed: %s (%s): %v", plugin.Name(), plugin.ID(), err)
		}
	}

	m.ctx = nil
	m.cancel = nil
}

func (m *Manager) Statuses() []Status {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	statuses := make([]Status, 0, len(m.plugins))
	for _, plugin := range m.plugins {
		statuses = append(statuses, plugin.Status())
	}
	return statuses
}

func (m *Manager) findLocked(pluginID string) Plugin {
	for _, plugin := range m.plugins {
		if plugin.ID() == pluginID {
			return plugin
		}
	}
	return nil
}

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
