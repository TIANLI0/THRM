package types

/*
散热收益 (Cooling Benefit)。

回答一个学习模式回答不了的问题：这台散热器在这台机器上，多吹 1000 转到底换来了
什么？是低了几度，还是多释放了几瓦？哪个零部件受益最大？

它与自适应学习完全分离——独立的配置分支、独立的观测通路、独立的分析代码。学习
关心的是"该吹多少转"，收益关心的是"吹到这个转速值不值"，两者的数据口径和可信度
要求都不一样，混在一起只会让哪一边都说不清楚。

数据只有一个来源：主动扫描测试。用户挂着同一个负载，程序把转速逐档锁定并等热稳定
后采样，同负载横向对比，结论可以直接归因给散热器。曾经还有一条"日常被动统计"的路径，
但那些样本来自不同时间的不同负载，即便按功耗分桶也只能标注"仅供参考"——它带来的
解释负担超过了它的价值，已经移除。
*/

// 散热收益扫描测试的转速范围与档位。热稳定远慢于声学稳定，因此档位比噪音测试稀疏得多：
// 一次测试已经要 8~12 分钟，再密下去用户不会有耐心跑完。
const (
	CoolingBenefitMinRPM  = 1000
	CoolingBenefitMaxRPM  = 4000
	CoolingBenefitStepRPM = 600
)

// 散热收益的受限形态。同一台机器换个负载就可能切换，因此必须按次判定而不是写死。
const (
	// CoolingRegimeThermal 温度墙：温度顶住上限不动，多吹的转速换成了更高的可持续功耗。
	CoolingRegimeThermal = "thermal"
	// CoolingRegimePower 功耗墙：功耗恒定，多吹的转速换成了更低的温度。
	CoolingRegimePower = "power"
	// CoolingRegimeMixed 温度和功耗同时改善。
	CoolingRegimeMixed = "mixed"
	// CoolingRegimeInconclusive 两者都没有明显变化：负载太轻，或这台机器吃不到散热器的收益。
	CoolingRegimeInconclusive = "inconclusive"
)

// 结果可信度的告警码。用码而不是句子，是为了让前端能本地化。
const (
	CoolingWarnLoadUnstable   = "loadUnstable"   // 各档功耗差异过大，负载中途变了
	CoolingWarnLoadTooLight   = "loadTooLight"   // 负载太轻，测不出区分度
	CoolingWarnNotSettled     = "notSettled"     // 某些档位采样窗口内仍在漂移
	CoolingWarnRPMUnreachable = "rpmUnreachable" // 风扇达不到目标转速
	CoolingWarnFewSteps       = "fewSteps"       // 有效档位太少
)

// CoolingSensorReading 是某个具名传感器在一个转速档上的稳态读数。
// 逐传感器记录是为了回答"哪个零部件受益最大"——整机平均温度会把
// GPU 热点降 12°C 和 CPU 只降 2°C 抹成一个没有意义的中间数。
type CoolingSensorReading struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Group string  `json:"group"` // cpu / gpu
	Value float64 `json:"value"` // °C
}

// CoolingBenefitStep 是扫描测试中一个转速档位的稳态结果。
type CoolingBenefitStep struct {
	TargetRPM int `json:"targetRpm"` // 下发的转速
	ActualRPM int `json:"actualRpm"` // 采样期间设备回报的实际转速

	CPUTemp  float64 `json:"cpuTemp"`
	GPUTemp  float64 `json:"gpuTemp"`
	CPUPower float64 `json:"cpuPower"`
	GPUPower float64 `json:"gpuPower"`

	LaptopCPUFanRPM int `json:"laptopCpuFanRpm"` // 本机内置风扇转速，0 表示读不到
	LaptopGPUFanRPM int `json:"laptopGpuFanRpm"`

	Sensors []CoolingSensorReading `json:"sensors"`

	Samples    int     `json:"samples"`    // 该档位的采样点数
	TempRange  float64 `json:"tempRange"`  // 采样窗口内的温度波动，用于判断是否真的稳了
	PowerRange float64 `json:"powerRange"` // 同上，功耗波动
}

// CoolingSensorDelta 是单个传感器从基准档到最高档的变化。
type CoolingSensorDelta struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Group    string  `json:"group"`
	Baseline float64 `json:"baseline"` // 基准档读数 (°C)
	Best     float64 `json:"best"`     // 最高档读数 (°C)
	Delta    float64 `json:"delta"`    // Best - Baseline，负值表示降温
}

// CoolingBenefitAnalysis 是对一次扫描结果的解读。
type CoolingBenefitAnalysis struct {
	Regime string `json:"regime"`

	BaselineRPM int `json:"baselineRpm"`
	TopRPM      int `json:"topRpm"`

	TempDelta       float64 `json:"tempDelta"`       // 控温温度变化 (°C)，负值为降温
	PowerDelta      float64 `json:"powerDelta"`      // 总功耗变化 (W)，正值为性能释放
	LaptopFanDelta  int     `json:"laptopFanDelta"`  // 本机风扇转速变化 (RPM)，负值为本机风扇得以降速
	TempPerKiloRPM  float64 `json:"tempPerKiloRpm"`  // 每 1000 RPM 的降温 (°C)
	PowerPerKiloRPM float64 `json:"powerPerKiloRpm"` // 每 1000 RPM 的功耗释放 (W)

	// KneeRPM 是收益拐点：越过它继续提速，换来的改善明显变小。
	KneeRPM int `json:"kneeRpm"`
	// SweetSpotRPM 在有噪音实测档案时给出"每分贝换到最多收益"的转速；无档案时等于 KneeRPM。
	SweetSpotRPM      int  `json:"sweetSpotRpm"`
	SweetSpotHasNoise bool `json:"sweetSpotHasNoise"`

	SensorDeltas []CoolingSensorDelta `json:"sensorDeltas"` // 按收益从大到小排序
	Warnings     []string             `json:"warnings"`
}

// CoolingBenefitReport 是一次完整的主动扫描测试。
type CoolingBenefitReport struct {
	CreatedAt   int64  `json:"createdAt"`
	DeviceModel string `json:"deviceModel"`
	CPUModel    string `json:"cpuModel"`
	GPUModel    string `json:"gpuModel"`
	LoadLabel   string `json:"loadLabel"` // 用户自填的负载说明，便于日后对照

	Steps    []CoolingBenefitStep   `json:"steps"`
	Analysis CoolingBenefitAnalysis `json:"analysis"`
}

// CoolingBenefitState 是散热收益功能的全部持久化状态。
type CoolingBenefitState struct {
	Report *CoolingBenefitReport `json:"report,omitempty"`
}

// CoolingBenefitPayload 是散热收益面板需要的全部数据。
type CoolingBenefitPayload struct {
	Report *CoolingBenefitReport `json:"report"`
}
