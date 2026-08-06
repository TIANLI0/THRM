//go:build !windows

package rtss

import "errors"

var (
	ErrRTSSNotInstalled = errors.New("RTSS 布局助手仅支持 Windows")
)

func InspectLayout() LayoutStatus {
	return LayoutStatus{Supported: false, AnchorIndex: -1}
}

func CreateAnchor() (LayoutStatus, error) {
	return InspectLayout(), ErrRTSSNotInstalled
}
