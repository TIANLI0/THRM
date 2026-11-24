package bridge

func (m *Manager) IsSupported() bool {
	return isBridgeSupported()
}
