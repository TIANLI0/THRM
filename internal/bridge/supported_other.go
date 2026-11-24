//go:build !windows

package bridge

func isBridgeSupported() bool {
	return false
}
