//go:build !windows

package flydigicompat

import "github.com/TIANLI0/THRM/internal/types"

// Detect 非 Windows 平台上飞智空间站不存在，恒返回不支持。
func Detect(_ types.Logger) Status {
	return Status{}
}

// NeedsApply 非 Windows 平台上永远无需处理。
func NeedsApply(_ types.Logger) bool {
	return false
}

// Apply 非 Windows 平台上是空操作。
func Apply(_ types.Logger, _ string) (Status, error) {
	return Status{}, nil
}

// Revert 非 Windows 平台上是空操作。
func Revert(_ types.Logger, _ string) (Status, error) {
	return Status{}, nil
}
