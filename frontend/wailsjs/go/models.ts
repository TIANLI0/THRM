export namespace flydigicompat {
	
	export class Status {
	    supported: boolean;
	    serviceInstalled: boolean;
	    serviceRunning: boolean;
	    totalNodes: number;
	    appliedNodes: number;
	    presentNodes: number;
	    effective?: boolean;
	    needsReconnect: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.serviceInstalled = source["serviceInstalled"];
	        this.serviceRunning = source["serviceRunning"];
	        this.totalNodes = source["totalNodes"];
	        this.appliedNodes = source["appliedNodes"];
	        this.presentNodes = source["presentNodes"];
	        this.effective = source["effective"];
	        this.needsReconnect = source["needsReconnect"];
	        this.error = source["error"];
	    }
	}

}

export namespace guiapp {
	
	export class WindowState {
	    width: number;
	    height: number;
	    x: number;
	    y: number;
	    maximised: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WindowState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.maximised = source["maximised"];
	    }
	}

}

export namespace ipc {
	
	export class SaveCoolingBenefitReportParams {
	    deviceModel: string;
	    cpuModel: string;
	    gpuModel: string;
	    loadLabel: string;
	    steps: types.CoolingBenefitStep[];
	
	    static createFrom(source: any = {}) {
	        return new SaveCoolingBenefitReportParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceModel = source["deviceModel"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.loadLabel = source["loadLabel"];
	        this.steps = this.convertValues(source["steps"], types.CoolingBenefitStep);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace rtss {
	
	export class LayoutStatus {
	    supported: boolean;
	    installed: boolean;
	    installPath: string;
	    configPath: string;
	    layoutPath: string;
	    layoutName: string;
	    backupPath: string;
	    anchorState: string;
	    anchorIndex: number;
	    layerCount: number;
	
	    static createFrom(source: any = {}) {
	        return new LayoutStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.installed = source["installed"];
	        this.installPath = source["installPath"];
	        this.configPath = source["configPath"];
	        this.layoutPath = source["layoutPath"];
	        this.layoutName = source["layoutName"];
	        this.backupPath = source["backupPath"];
	        this.anchorState = source["anchorState"];
	        this.anchorIndex = source["anchorIndex"];
	        this.layerCount = source["layerCount"];
	    }
	}

}

export namespace smartcontrol {
	
	export class AdaptiveStatus {
	    enabled: boolean;
	    preference: number;
	    tempLimit: number;
	    ceilingTemp: number;
	    rpmFloor: number;
	    rpmCeil: number;
	    confidence: number;
	    samples: number;
	    baseline: number;
	    usingPower: boolean;
	    curve: types.FanCurvePoint[];
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AdaptiveStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.preference = source["preference"];
	        this.tempLimit = source["tempLimit"];
	        this.ceilingTemp = source["ceilingTemp"];
	        this.rpmFloor = source["rpmFloor"];
	        this.rpmCeil = source["rpmCeil"];
	        this.confidence = source["confidence"];
	        this.samples = source["samples"];
	        this.baseline = source["baseline"];
	        this.usingPower = source["usingPower"];
	        this.curve = this.convertValues(source["curve"], types.FanCurvePoint);
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace theme {
	
	export class Meta {
	    id: string;
	    name: string;
	    base: string;
	    author?: string;
	    version?: string;
	    description?: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.base = source["base"];
	        this.author = source["author"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.source = source["source"];
	    }
	}

}

export namespace types {
	
	export class FanCurvePoint {
	    temperature: number;
	    rpm: number;
	
	    static createFrom(source: any = {}) {
	        return new FanCurvePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.temperature = source["temperature"];
	        this.rpm = source["rpm"];
	    }
	}
	export class AdaptiveThermalBucket {
	    rpm: number;
	    risePerWatt: number;
	    rise: number;
	    weight: number;
	    powerWeight: number;
	
	    static createFrom(source: any = {}) {
	        return new AdaptiveThermalBucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rpm = source["rpm"];
	        this.risePerWatt = source["risePerWatt"];
	        this.rise = source["rise"];
	        this.weight = source["weight"];
	        this.powerWeight = source["powerWeight"];
	    }
	}
	export class AdaptiveThermalModel {
	    baseline: number;
	    buckets: AdaptiveThermalBucket[];
	    maxObservedRpm: number;
	    samples: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AdaptiveThermalModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseline = source["baseline"];
	        this.buckets = this.convertValues(source["buckets"], AdaptiveThermalBucket);
	        this.maxObservedRpm = source["maxObservedRpm"];
	        this.samples = source["samples"];
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AdaptiveConfig {
	    enabled: boolean;
	    preference: number;
	    tempLimit: number;
	    model: AdaptiveThermalModel;
	    autoCurve: FanCurvePoint[];
	    autoCurveUpdatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AdaptiveConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.preference = source["preference"];
	        this.tempLimit = source["tempLimit"];
	        this.model = this.convertValues(source["model"], AdaptiveThermalModel);
	        this.autoCurve = this.convertValues(source["autoCurve"], FanCurvePoint);
	        this.autoCurveUpdatedAt = source["autoCurveUpdatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class RGBColor {
	    r: number;
	    g: number;
	    b: number;
	
	    static createFrom(source: any = {}) {
	        return new RGBColor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.r = source["r"];
	        this.g = source["g"];
	        this.b = source["b"];
	    }
	}
	export class LightStripConfig {
	    mode: string;
	    speed: string;
	    brightness: number;
	    colors: RGBColor[];
	
	    static createFrom(source: any = {}) {
	        return new LightStripConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.speed = source["speed"];
	        this.brightness = source["brightness"];
	        this.colors = this.convertValues(source["colors"], RGBColor);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CoolingSensorDelta {
	    key: string;
	    name: string;
	    group: string;
	    baseline: number;
	    best: number;
	    delta: number;
	
	    static createFrom(source: any = {}) {
	        return new CoolingSensorDelta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.baseline = source["baseline"];
	        this.best = source["best"];
	        this.delta = source["delta"];
	    }
	}
	export class CoolingBenefitAnalysis {
	    regime: string;
	    baselineRpm: number;
	    topRpm: number;
	    tempDelta: number;
	    powerDelta: number;
	    laptopFanDelta: number;
	    tempPerKiloRpm: number;
	    powerPerKiloRpm: number;
	    kneeRpm: number;
	    sweetSpotRpm: number;
	    sweetSpotHasNoise: boolean;
	    sensorDeltas: CoolingSensorDelta[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new CoolingBenefitAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.regime = source["regime"];
	        this.baselineRpm = source["baselineRpm"];
	        this.topRpm = source["topRpm"];
	        this.tempDelta = source["tempDelta"];
	        this.powerDelta = source["powerDelta"];
	        this.laptopFanDelta = source["laptopFanDelta"];
	        this.tempPerKiloRpm = source["tempPerKiloRpm"];
	        this.powerPerKiloRpm = source["powerPerKiloRpm"];
	        this.kneeRpm = source["kneeRpm"];
	        this.sweetSpotRpm = source["sweetSpotRpm"];
	        this.sweetSpotHasNoise = source["sweetSpotHasNoise"];
	        this.sensorDeltas = this.convertValues(source["sensorDeltas"], CoolingSensorDelta);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CoolingSensorReading {
	    key: string;
	    name: string;
	    group: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new CoolingSensorReading(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.value = source["value"];
	    }
	}
	export class CoolingBenefitStep {
	    targetRpm: number;
	    actualRpm: number;
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPower: number;
	    gpuPower: number;
	    laptopCpuFanRpm: number;
	    laptopGpuFanRpm: number;
	    sensors: CoolingSensorReading[];
	    samples: number;
	    tempRange: number;
	    powerRange: number;
	
	    static createFrom(source: any = {}) {
	        return new CoolingBenefitStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetRpm = source["targetRpm"];
	        this.actualRpm = source["actualRpm"];
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPower = source["cpuPower"];
	        this.gpuPower = source["gpuPower"];
	        this.laptopCpuFanRpm = source["laptopCpuFanRpm"];
	        this.laptopGpuFanRpm = source["laptopGpuFanRpm"];
	        this.sensors = this.convertValues(source["sensors"], CoolingSensorReading);
	        this.samples = source["samples"];
	        this.tempRange = source["tempRange"];
	        this.powerRange = source["powerRange"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CoolingBenefitReport {
	    createdAt: number;
	    deviceModel: string;
	    cpuModel: string;
	    gpuModel: string;
	    loadLabel: string;
	    steps: CoolingBenefitStep[];
	    analysis: CoolingBenefitAnalysis;
	
	    static createFrom(source: any = {}) {
	        return new CoolingBenefitReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.createdAt = source["createdAt"];
	        this.deviceModel = source["deviceModel"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.loadLabel = source["loadLabel"];
	        this.steps = this.convertValues(source["steps"], CoolingBenefitStep);
	        this.analysis = this.convertValues(source["analysis"], CoolingBenefitAnalysis);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CoolingBenefitState {
	    report?: CoolingBenefitReport;
	
	    static createFrom(source: any = {}) {
	        return new CoolingBenefitState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = this.convertValues(source["report"], CoolingBenefitReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NoiseProfilePoint {
	    rpm: number;
	    db: number;
	
	    static createFrom(source: any = {}) {
	        return new NoiseProfilePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rpm = source["rpm"];
	        this.db = source["db"];
	    }
	}
	export class SmartControlConfig {
	    enabled: boolean;
	    learning: boolean;
	    predictiveBoost: boolean;
	    learningBias: string;
	    filterTransientSpike: boolean;
	    laptopFanGuard: boolean;
	    targetTemp: number;
	    aggressiveness: number;
	    hysteresis: number;
	    minRpmChange: number;
	    rampUpLimit: number;
	    rampDownLimit: number;
	    learnRate: number;
	    learnWindow: number;
	    learnDelay: number;
	    overheatWeight: number;
	    rpmDeltaWeight: number;
	    noiseWeight: number;
	    trendGain: number;
	    maxLearnOffset: number;
	    learnedOffsets: number[];
	    learnedOffsetsByProfile: Record<string, Array<number>>;
	    targetTempByProfile?: Record<string, number>;
	    learningBiasByProfile?: Record<string, string>;
	    learnedOffsetsHeat: number[];
	    learnedOffsetsCool: number[];
	    learnedRateHeat: number[];
	    learnedRateCool: number[];
	    noiseProfile: NoiseProfilePoint[];
	    noiseProfileUpdatedAt: number;
	    adaptive: AdaptiveConfig;
	
	    static createFrom(source: any = {}) {
	        return new SmartControlConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.learning = source["learning"];
	        this.predictiveBoost = source["predictiveBoost"];
	        this.learningBias = source["learningBias"];
	        this.filterTransientSpike = source["filterTransientSpike"];
	        this.laptopFanGuard = source["laptopFanGuard"];
	        this.targetTemp = source["targetTemp"];
	        this.aggressiveness = source["aggressiveness"];
	        this.hysteresis = source["hysteresis"];
	        this.minRpmChange = source["minRpmChange"];
	        this.rampUpLimit = source["rampUpLimit"];
	        this.rampDownLimit = source["rampDownLimit"];
	        this.learnRate = source["learnRate"];
	        this.learnWindow = source["learnWindow"];
	        this.learnDelay = source["learnDelay"];
	        this.overheatWeight = source["overheatWeight"];
	        this.rpmDeltaWeight = source["rpmDeltaWeight"];
	        this.noiseWeight = source["noiseWeight"];
	        this.trendGain = source["trendGain"];
	        this.maxLearnOffset = source["maxLearnOffset"];
	        this.learnedOffsets = source["learnedOffsets"];
	        this.learnedOffsetsByProfile = source["learnedOffsetsByProfile"];
	        this.targetTempByProfile = source["targetTempByProfile"];
	        this.learningBiasByProfile = source["learningBiasByProfile"];
	        this.learnedOffsetsHeat = source["learnedOffsetsHeat"];
	        this.learnedOffsetsCool = source["learnedOffsetsCool"];
	        this.learnedRateHeat = source["learnedRateHeat"];
	        this.learnedRateCool = source["learnedRateCool"];
	        this.noiseProfile = this.convertValues(source["noiseProfile"], NoiseProfilePoint);
	        this.noiseProfileUpdatedAt = source["noiseProfileUpdatedAt"];
	        this.adaptive = this.convertValues(source["adaptive"], AdaptiveConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TimeCurveScheduleRule {
	    id: string;
	    name: string;
	    enabled: boolean;
	    weekdays: number[];
	    startTime: string;
	    endTime: string;
	    curveProfileId: string;
	
	    static createFrom(source: any = {}) {
	        return new TimeCurveScheduleRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.weekdays = source["weekdays"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.curveProfileId = source["curveProfileId"];
	    }
	}
	export class TimeCurveScheduleConfig {
	    enabled: boolean;
	    rules: TimeCurveScheduleRule[];
	
	    static createFrom(source: any = {}) {
	        return new TimeCurveScheduleConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.rules = this.convertValues(source["rules"], TimeCurveScheduleRule);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SpeedAvoidanceConfig {
	    enabled: boolean;
	    minRpm: number;
	    maxRpm: number;
	    marginRpm: number;
	    emergencyBypassTemp: number;
	
	    static createFrom(source: any = {}) {
	        return new SpeedAvoidanceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.minRpm = source["minRpm"];
	        this.maxRpm = source["maxRpm"];
	        this.marginRpm = source["marginRpm"];
	        this.emergencyBypassTemp = source["emergencyBypassTemp"];
	    }
	}
	export class RTSSConfig {
	    enabled: boolean;
	    updateIntervalMs: number;
	    positionMode: string;
	    positionX: number;
	    positionY: number;
	
	    static createFrom(source: any = {}) {
	        return new RTSSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.updateIntervalMs = source["updateIntervalMs"];
	        this.positionMode = source["positionMode"];
	        this.positionX = source["positionX"];
	        this.positionY = source["positionY"];
	    }
	}
	export class FanCurveProfile {
	    id: string;
	    name: string;
	    curve: FanCurvePoint[];
	
	    static createFrom(source: any = {}) {
	        return new FanCurveProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.curve = this.convertValues(source["curve"], FanCurvePoint);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LegionFnQSupportCache {
	    checked: boolean;
	    supported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LegionFnQSupportCache(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.supported = source["supported"];
	    }
	}
	export class FanGearTarget {
	    gear: string;
	    level: string;
	
	    static createFrom(source: any = {}) {
	        return new FanGearTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gear = source["gear"];
	        this.level = source["level"];
	    }
	}
	export class LegionFnQConfig {
	    enabled: boolean;
	    takeOverFan: boolean;
	    modeMapping: Record<string, FanGearTarget>;
	
	    static createFrom(source: any = {}) {
	        return new LegionFnQConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.takeOverFan = source["takeOverFan"];
	        this.modeMapping = this.convertValues(source["modeMapping"], FanGearTarget, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppConfig {
	    legionFnQ: LegionFnQConfig;
	    legionFnQSupport: LegionFnQSupportCache;
	    autoControl: boolean;
	    manualGearToggleHotkey: string;
	    autoControlToggleHotkey: string;
	    curveProfileToggleHotkey: string;
	    manualGearLevels: Record<string, string>;
	    manualGearRpm: Record<string, any>;
	    fanCurve: FanCurvePoint[];
	    fanCurveProfiles: FanCurveProfile[];
	    activeFanCurveProfileId: string;
	    gearLight: boolean;
	    powerOnStart: boolean;
	    windowsAutoStart: boolean;
	    disableSystemTray: boolean;
	    themeMode: string;
	    smartStartStop: string;
	    brightness: number;
	    tempUpdateRate: number;
	    tempSampleCount: number;
	    tempSource: string;
	    temperatureHistoryRetentionHours: number;
	    disableGpuMonitoring: boolean;
	    gpuDevice: string;
	    cpuSensor: string;
	    cpuSensors: string[];
	    gpuSensor: string;
	    windowBlur: string;
	    configPath: string;
	    manualGear: string;
	    manualLevel: string;
	    debugMode: boolean;
	    guiMonitoring: boolean;
	    customSpeedEnabled: boolean;
	    customSpeedRPM: number;
	    ignoreDeviceOnReconnect: boolean;
	    lastDeviceTransport: string;
	    flydigiCompat: boolean;
	    rtss: RTSSConfig;
	    speedAvoidance: SpeedAvoidanceConfig;
	    timeCurveSchedule: TimeCurveScheduleConfig;
	    smartControl: SmartControlConfig;
	    coolingBenefit: CoolingBenefitState;
	    lightStrip: LightStripConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.legionFnQ = this.convertValues(source["legionFnQ"], LegionFnQConfig);
	        this.legionFnQSupport = this.convertValues(source["legionFnQSupport"], LegionFnQSupportCache);
	        this.autoControl = source["autoControl"];
	        this.manualGearToggleHotkey = source["manualGearToggleHotkey"];
	        this.autoControlToggleHotkey = source["autoControlToggleHotkey"];
	        this.curveProfileToggleHotkey = source["curveProfileToggleHotkey"];
	        this.manualGearLevels = source["manualGearLevels"];
	        this.manualGearRpm = source["manualGearRpm"];
	        this.fanCurve = this.convertValues(source["fanCurve"], FanCurvePoint);
	        this.fanCurveProfiles = this.convertValues(source["fanCurveProfiles"], FanCurveProfile);
	        this.activeFanCurveProfileId = source["activeFanCurveProfileId"];
	        this.gearLight = source["gearLight"];
	        this.powerOnStart = source["powerOnStart"];
	        this.windowsAutoStart = source["windowsAutoStart"];
	        this.disableSystemTray = source["disableSystemTray"];
	        this.themeMode = source["themeMode"];
	        this.smartStartStop = source["smartStartStop"];
	        this.brightness = source["brightness"];
	        this.tempUpdateRate = source["tempUpdateRate"];
	        this.tempSampleCount = source["tempSampleCount"];
	        this.tempSource = source["tempSource"];
	        this.temperatureHistoryRetentionHours = source["temperatureHistoryRetentionHours"];
	        this.disableGpuMonitoring = source["disableGpuMonitoring"];
	        this.gpuDevice = source["gpuDevice"];
	        this.cpuSensor = source["cpuSensor"];
	        this.cpuSensors = source["cpuSensors"];
	        this.gpuSensor = source["gpuSensor"];
	        this.windowBlur = source["windowBlur"];
	        this.configPath = source["configPath"];
	        this.manualGear = source["manualGear"];
	        this.manualLevel = source["manualLevel"];
	        this.debugMode = source["debugMode"];
	        this.guiMonitoring = source["guiMonitoring"];
	        this.customSpeedEnabled = source["customSpeedEnabled"];
	        this.customSpeedRPM = source["customSpeedRPM"];
	        this.ignoreDeviceOnReconnect = source["ignoreDeviceOnReconnect"];
	        this.lastDeviceTransport = source["lastDeviceTransport"];
	        this.flydigiCompat = source["flydigiCompat"];
	        this.rtss = this.convertValues(source["rtss"], RTSSConfig);
	        this.speedAvoidance = this.convertValues(source["speedAvoidance"], SpeedAvoidanceConfig);
	        this.timeCurveSchedule = this.convertValues(source["timeCurveSchedule"], TimeCurveScheduleConfig);
	        this.smartControl = this.convertValues(source["smartControl"], SmartControlConfig);
	        this.coolingBenefit = this.convertValues(source["coolingBenefit"], CoolingBenefitState);
	        this.lightStrip = this.convertValues(source["lightStrip"], LightStripConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TemperatureGPUDevice {
	    key: string;
	    name: string;
	    vendor: string;
	    sensors: TemperatureSensor[];
	    powerSensors: PowerSensor[];
	
	    static createFrom(source: any = {}) {
	        return new TemperatureGPUDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.vendor = source["vendor"];
	        this.sensors = this.convertValues(source["sensors"], TemperatureSensor);
	        this.powerSensors = this.convertValues(source["powerSensors"], PowerSensor);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PowerSensor {
	    key: string;
	    name: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new PowerSensor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class TemperatureSensor {
	    key: string;
	    name: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new TemperatureSensor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class BridgeTemperatureData {
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPower: number;
	    gpuPower: number;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
	    cpuPowerSensors: PowerSensor[];
	    gpuPowerSensors: PowerSensor[];
	    gpuDevices: TemperatureGPUDevice[];
	    otherSensors: TemperatureSensor[];
	    updateTime: number;
	    success: boolean;
	    error: string;
	    cpuTempError: string;
	
	    static createFrom(source: any = {}) {
	        return new BridgeTemperatureData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPower = source["cpuPower"];
	        this.gpuPower = source["gpuPower"];
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
	        this.cpuPowerSensors = this.convertValues(source["cpuPowerSensors"], PowerSensor);
	        this.gpuPowerSensors = this.convertValues(source["gpuPowerSensors"], PowerSensor);
	        this.gpuDevices = this.convertValues(source["gpuDevices"], TemperatureGPUDevice);
	        this.otherSensors = this.convertValues(source["otherSensors"], TemperatureSensor);
	        this.updateTime = source["updateTime"];
	        this.success = source["success"];
	        this.error = source["error"];
	        this.cpuTempError = source["cpuTempError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class CoolingBenefitPayload {
	    report?: CoolingBenefitReport;
	
	    static createFrom(source: any = {}) {
	        return new CoolingBenefitPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = this.convertValues(source["report"], CoolingBenefitReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	export class DeviceDebugFrame {
	    id: number;
	    direction: string;
	    transport: string;
	    timestamp: string;
	    rawHex: string;
	    frameHex: string;
	    command: string;
	    length: number;
	    payloadHex: string;
	    checksumOk: boolean;
	    description: string;
	    decoded?: string;
	    parsed?: any;
	
	    static createFrom(source: any = {}) {
	        return new DeviceDebugFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.direction = source["direction"];
	        this.transport = source["transport"];
	        this.timestamp = source["timestamp"];
	        this.rawHex = source["rawHex"];
	        this.frameHex = source["frameHex"];
	        this.command = source["command"];
	        this.length = source["length"];
	        this.payloadHex = source["payloadHex"];
	        this.checksumOk = source["checksumOk"];
	        this.description = source["description"];
	        this.decoded = source["decoded"];
	        this.parsed = source["parsed"];
	    }
	}
	export class DeviceDebugCommandResult {
	    transport: string;
	    inputHex: string;
	    frameHex: string;
	    rawHex: string;
	    waitMs: number;
	    frames: DeviceDebugFrame[];
	
	    static createFrom(source: any = {}) {
	        return new DeviceDebugCommandResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.transport = source["transport"];
	        this.inputHex = source["inputHex"];
	        this.frameHex = source["frameHex"];
	        this.rawHex = source["rawHex"];
	        this.waitMs = source["waitMs"];
	        this.frames = this.convertValues(source["frames"], DeviceDebugFrame);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DeviceGearRPM {
	    gear: number;
	    label: string;
	    rpm: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceGearRPM(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gear = source["gear"];
	        this.label = source["label"];
	        this.rpm = source["rpm"];
	    }
	}
	export class DeviceStatusRead {
	    gearSetting?: string;
	    maxGear?: string;
	    selected?: string;
	    mode?: string;
	    modeName?: string;
	    smartStartStop?: string;
	    smartStartStopName?: string;
	    currentRpm?: number;
	    targetRpm?: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStatusRead(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gearSetting = source["gearSetting"];
	        this.maxGear = source["maxGear"];
	        this.selected = source["selected"];
	        this.mode = source["mode"];
	        this.modeName = source["modeName"];
	        this.smartStartStop = source["smartStartStop"];
	        this.smartStartStopName = source["smartStartStopName"];
	        this.currentRpm = source["currentRpm"];
	        this.targetRpm = source["targetRpm"];
	    }
	}
	export class DeviceSettings {
	    available: boolean;
	    source: string;
	    readAt: string;
	    readErrors?: string[];
	    model?: string;
	    deviceCpuModel?: string;
	    deviceCpuModelSource?: string;
	    hidManufacturer?: string;
	    hidProduct?: string;
	    hidSerialNumber?: string;
	    hidReleaseNumber?: number;
	    hidReleaseNumberHex?: string;
	    firmwareVersion?: string;
	    firmwareVersionRaw?: string;
	    firmwareReadStatus?: string;
	    firmwareReadError?: string;
	    deviceIdentifier?: string;
	    identityMarker?: string;
	    identityHex?: string;
	    configState?: string;
	    configStateName?: string;
	    controllerCapabilityTier?: number;
	    runtimeProfileRaw?: number;
	    measuredRpm?: number;
	    targetRpm?: number;
	    gearRpmTable?: DeviceGearRPM[];
	    queriedWorkState?: string;
	    queriedWorkStateName?: string;
	    liveModeFlags?: string;
	    liveModeName?: string;
	    activeGear?: number;
	    selectedGear?: number;
	    realtimeActive?: boolean;
	    rgbState?: string;
	    rgbStateName?: string;
	    status?: DeviceStatusRead;
	    rawFrames?: DeviceDebugFrame[];
	
	    static createFrom(source: any = {}) {
	        return new DeviceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.source = source["source"];
	        this.readAt = source["readAt"];
	        this.readErrors = source["readErrors"];
	        this.model = source["model"];
	        this.deviceCpuModel = source["deviceCpuModel"];
	        this.deviceCpuModelSource = source["deviceCpuModelSource"];
	        this.hidManufacturer = source["hidManufacturer"];
	        this.hidProduct = source["hidProduct"];
	        this.hidSerialNumber = source["hidSerialNumber"];
	        this.hidReleaseNumber = source["hidReleaseNumber"];
	        this.hidReleaseNumberHex = source["hidReleaseNumberHex"];
	        this.firmwareVersion = source["firmwareVersion"];
	        this.firmwareVersionRaw = source["firmwareVersionRaw"];
	        this.firmwareReadStatus = source["firmwareReadStatus"];
	        this.firmwareReadError = source["firmwareReadError"];
	        this.deviceIdentifier = source["deviceIdentifier"];
	        this.identityMarker = source["identityMarker"];
	        this.identityHex = source["identityHex"];
	        this.configState = source["configState"];
	        this.configStateName = source["configStateName"];
	        this.controllerCapabilityTier = source["controllerCapabilityTier"];
	        this.runtimeProfileRaw = source["runtimeProfileRaw"];
	        this.measuredRpm = source["measuredRpm"];
	        this.targetRpm = source["targetRpm"];
	        this.gearRpmTable = this.convertValues(source["gearRpmTable"], DeviceGearRPM);
	        this.queriedWorkState = source["queriedWorkState"];
	        this.queriedWorkStateName = source["queriedWorkStateName"];
	        this.liveModeFlags = source["liveModeFlags"];
	        this.liveModeName = source["liveModeName"];
	        this.activeGear = source["activeGear"];
	        this.selectedGear = source["selectedGear"];
	        this.realtimeActive = source["realtimeActive"];
	        this.rgbState = source["rgbState"];
	        this.rgbStateName = source["rgbStateName"];
	        this.status = this.convertValues(source["status"], DeviceStatusRead);
	        this.rawFrames = this.convertValues(source["rawFrames"], DeviceDebugFrame);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class FanCurveProfilesPayload {
	    profiles: FanCurveProfile[];
	    activeId: string;
	
	    static createFrom(source: any = {}) {
	        return new FanCurveProfilesPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], FanCurveProfile);
	        this.activeId = source["activeId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FanData {
	    reportId: number;
	    magicSync: number;
	    command: number;
	    frameLength: number;
	    status?: number;
	    gearSettings: number;
	    currentMode: number;
	    reserved1: number;
	    currentRpm: number;
	    targetRpm: number;
	    maxGear: string;
	    setGear: string;
	    workMode: string;
	
	    static createFrom(source: any = {}) {
	        return new FanData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reportId = source["reportId"];
	        this.magicSync = source["magicSync"];
	        this.command = source["command"];
	        this.frameLength = source["frameLength"];
	        this.status = source["status"];
	        this.gearSettings = source["gearSettings"];
	        this.currentMode = source["currentMode"];
	        this.reserved1 = source["reserved1"];
	        this.currentRpm = source["currentRpm"];
	        this.targetRpm = source["targetRpm"];
	        this.maxGear = source["maxGear"];
	        this.setGear = source["setGear"];
	        this.workMode = source["workMode"];
	    }
	}
	
	
	
	
	
	
	
	
	
	
	export class TemperatureData {
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPower: number;
	    gpuPower: number;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
	    cpuPowerSensors: PowerSensor[];
	    gpuPowerSensors: PowerSensor[];
	    gpuDevices: TemperatureGPUDevice[];
	    otherSensors?: TemperatureSensor[];
	    updateTime: number;
	    bridgeOk: boolean;
	    bridgeMessage: string;
	    cpuTempError: string;
	    cpuFanRpm: number;
	    gpuFanRpm: number;
	
	    static createFrom(source: any = {}) {
	        return new TemperatureData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPower = source["cpuPower"];
	        this.gpuPower = source["gpuPower"];
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
	        this.cpuPowerSensors = this.convertValues(source["cpuPowerSensors"], PowerSensor);
	        this.gpuPowerSensors = this.convertValues(source["gpuPowerSensors"], PowerSensor);
	        this.gpuDevices = this.convertValues(source["gpuDevices"], TemperatureGPUDevice);
	        this.otherSensors = this.convertValues(source["otherSensors"], TemperatureSensor);
	        this.updateTime = source["updateTime"];
	        this.bridgeOk = source["bridgeOk"];
	        this.bridgeMessage = source["bridgeMessage"];
	        this.cpuTempError = source["cpuTempError"];
	        this.cpuFanRpm = source["cpuFanRpm"];
	        this.gpuFanRpm = source["gpuFanRpm"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class TimelineEvent {
	    timestamp: number;
	    type: string;
	    labelKey: string;
	
	    static createFrom(source: any = {}) {
	        return new TimelineEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.labelKey = source["labelKey"];
	    }
	}
	export class TemperatureHistoryPoint {
	    timestamp: number;
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPower: number;
	    gpuPower: number;
	    fanRpm: number;
	    cpuFanRpm: number;
	    gpuFanRpm: number;
	
	    static createFrom(source: any = {}) {
	        return new TemperatureHistoryPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPower = source["cpuPower"];
	        this.gpuPower = source["gpuPower"];
	        this.fanRpm = source["fanRpm"];
	        this.cpuFanRpm = source["cpuFanRpm"];
	        this.gpuFanRpm = source["gpuFanRpm"];
	    }
	}
	export class TemperatureHistoryPayload {
	    enabled: boolean;
	    sampleIntervalSeconds: number;
	    retentionHours: number;
	    points: TemperatureHistoryPoint[];
	    events: TimelineEvent[];
	
	    static createFrom(source: any = {}) {
	        return new TemperatureHistoryPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.sampleIntervalSeconds = source["sampleIntervalSeconds"];
	        this.retentionHours = source["retentionHours"];
	        this.points = this.convertValues(source["points"], TemperatureHistoryPoint);
	        this.events = this.convertValues(source["events"], TimelineEvent);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	

}

