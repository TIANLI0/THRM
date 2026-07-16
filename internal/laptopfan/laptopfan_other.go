//go:build !windows

package laptopfan

import "github.com/TIANLI0/THRM/internal/types"

type stubReader struct{}

func newPlatformReader(_ types.Logger) readerImpl {
	return stubReader{}
}

func (stubReader) read() (FanSpeeds, bool) {
	return FanSpeeds{}, false
}
