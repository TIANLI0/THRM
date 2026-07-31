//go:build !windows

package rtss

type unavailableSink struct{}

func newSharedMemorySink() osdSink         { return unavailableSink{} }
func (unavailableSink) Update(uint16) bool { return false }
func (unavailableSink) Close()             {}
