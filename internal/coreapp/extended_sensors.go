package coreapp

import "time"

// extendedSensorsMaxWindow 是临时开启的最长时限。散热收益测试满打满算不到 20 分钟，
// 留出充裕余量即可；到期自动关闭是为了兜住"GUI 卡死或崩溃，没人来发关闭指令"的情况。
const extendedSensorsMaxWindow = 45 * time.Minute

// SetExtendedSensors 临时打开或关闭扩展温度传感器（内存 / 硬盘 / 主板 / EC / 电源）。
//
// 刻意不落配置：这些读数不参与控温，只有散热收益测试需要，而为它们打开的
// SMART/SPD/Super I/O 通道会一直消耗后台资源。开着的状态因此是会话级的，
// 核心重启、GUI 断开或超时都会让它回到关闭。
func (a *CoreApp) SetExtendedSensors(enabled bool) bool {
	if !enabled {
		a.extendedSensorsUntil.Store(0)
		a.logInfo("扩展温度传感器已关闭")
		return false
	}
	a.extendedSensorsUntil.Store(time.Now().Add(extendedSensorsMaxWindow).UnixMilli())
	a.logInfo("扩展温度传感器已临时开启，最长 %s", extendedSensorsMaxWindow)
	return true
}

// extendedSensorsActive 报告当前是否应当读取扩展传感器，并顺带处理超时自动关闭。
func (a *CoreApp) extendedSensorsActive() bool {
	until := a.extendedSensorsUntil.Load()
	if until == 0 {
		return false
	}
	if time.Now().UnixMilli() >= until {
		// CompareAndSwap 而不是直接 Store：避免把这中间刚发来的新一轮开启擦掉。
		if a.extendedSensorsUntil.CompareAndSwap(until, 0) {
			a.logInfo("扩展温度传感器已超时自动关闭")
		}
		return false
	}
	return true
}

// releaseExtendedSensorsIfIdle 在 GUI 断开后关闭扩展传感器。
// 测试跑在 GUI 里，GUI 一走测试必然已经结束或死掉，没有理由继续多轮询一批硬件。
func (a *CoreApp) releaseExtendedSensorsIfIdle(hasClients bool) {
	if hasClients || a.extendedSensorsUntil.Load() == 0 {
		return
	}
	a.extendedSensorsUntil.Store(0)
	a.logInfo("GUI 已断开，关闭扩展温度传感器")
}
