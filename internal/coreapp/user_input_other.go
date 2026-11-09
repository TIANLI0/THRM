//go:build !windows

package coreapp

// 非 Windows 平台没有区分自动唤醒的通知，也不需要输入时刻；一律报告"不可用"。
func currentInputTick() (uint32, bool)  { return 0, false }
func lastUserInputTick() (uint32, bool) { return 0, false }
