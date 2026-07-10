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
	    learningBias: string;
	    filterTransientSpike: boolean;
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
	
	    static createFrom(source: any = {}) {
	        return new SmartControlConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.learning = source["learning"];
	        this.learningBias = source["learningBias"];
	        this.filterTransientSpike = source["filterTransientSpike"];
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
	    themeMode: string;
	    smartStartStop: string;
	    brightness: number;
	    tempUpdateRate: number;
	    tempSampleCount: number;
	    tempSource: string;
	    gpuDevice: string;
	    cpuSensor: string;
	    cpuSensors: string[];
	    gpuSensor: string;
	    windowBlur: string;
	    suspendFanOff: boolean;
	    configPath: string;
	    manualGear: string;
	    manualLevel: string;
	    debugMode: boolean;
	    guiMonitoring: boolean;
	    customSpeedEnabled: boolean;
	    customSpeedRPM: number;
	    ignoreDeviceOnReconnect: boolean;
	    speedAvoidance: SpeedAvoidanceConfig;
	    timeCurveSchedule: TimeCurveScheduleConfig;
	    smartControl: SmartControlConfig;
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
	        this.themeMode = source["themeMode"];
	        this.smartStartStop = source["smartStartStop"];
	        this.brightness = source["brightness"];
	        this.tempUpdateRate = source["tempUpdateRate"];
	        this.tempSampleCount = source["tempSampleCount"];
	        this.tempSource = source["tempSource"];
	        this.gpuDevice = source["gpuDevice"];
	        this.cpuSensor = source["cpuSensor"];
	        this.cpuSensors = source["cpuSensors"];
	        this.gpuSensor = source["gpuSensor"];
	        this.windowBlur = source["windowBlur"];
	        this.suspendFanOff = source["suspendFanOff"];
	        this.configPath = source["configPath"];
	        this.manualGear = source["manualGear"];
	        this.manualLevel = source["manualLevel"];
	        this.debugMode = source["debugMode"];
	        this.guiMonitoring = source["guiMonitoring"];
	        this.customSpeedEnabled = source["customSpeedEnabled"];
	        this.customSpeedRPM = source["customSpeedRPM"];
	        this.ignoreDeviceOnReconnect = source["ignoreDeviceOnReconnect"];
	        this.speedAvoidance = this.convertValues(source["speedAvoidance"], SpeedAvoidanceConfig);
	        this.timeCurveSchedule = this.convertValues(source["timeCurveSchedule"], TimeCurveScheduleConfig);
	        this.smartControl = this.convertValues(source["smartControl"], SmartControlConfig);
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
	    updateTime: number;
	    success: boolean;
	    error: string;
	
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
	        this.updateTime = source["updateTime"];
	        this.success = source["success"];
	        this.error = source["error"];
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
	    model?: string;
	    gearRpmTable?: DeviceGearRPM[];
	    workMode?: string;
	    workModeName?: string;
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
	        this.model = source["model"];
	        this.gearRpmTable = this.convertValues(source["gearRpmTable"], DeviceGearRPM);
	        this.workMode = source["workMode"];
	        this.workModeName = source["workModeName"];
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
	    status: number;
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
	    updateTime: number;
	    bridgeOk: boolean;
	    bridgeMessage: string;
	
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
	        this.updateTime = source["updateTime"];
	        this.bridgeOk = source["bridgeOk"];
	        this.bridgeMessage = source["bridgeMessage"];
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
	
	export class TemperatureHistoryPoint {
	    timestamp: number;
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPower: number;
	    gpuPower: number;
	    fanRpm: number;
	
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
	    }
	}
	export class TemperatureHistoryPayload {
	    enabled: boolean;
	    sampleIntervalSeconds: number;
	    points: TemperatureHistoryPoint[];
	
	    static createFrom(source: any = {}) {
	        return new TemperatureHistoryPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.sampleIntervalSeconds = source["sampleIntervalSeconds"];
	        this.points = this.convertValues(source["points"], TemperatureHistoryPoint);
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
