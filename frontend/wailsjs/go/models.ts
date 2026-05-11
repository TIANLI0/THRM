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
	export class SmartControlConfig {
	    enabled: boolean;
	    learning: boolean;
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
	    learnedOffsetsHeat: number[];
	    learnedOffsetsCool: number[];
	    learnedRateHeat: number[];
	    learnedRateCool: number[];
	
	    static createFrom(source: any = {}) {
	        return new SmartControlConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.learning = source["learning"];
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
	        this.learnedOffsetsHeat = source["learnedOffsetsHeat"];
	        this.learnedOffsetsCool = source["learnedOffsetsCool"];
	        this.learnedRateHeat = source["learnedRateHeat"];
	        this.learnedRateCool = source["learnedRateCool"];
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
	    autoControl: boolean;
	    manualGearToggleHotkey: string;
	    autoControlToggleHotkey: string;
	    curveProfileToggleHotkey: string;
	    manualGearLevels: Record<string, string>;
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
	    gpuSensor: string;
	    configPath: string;
	    manualGear: string;
	    manualLevel: string;
	    debugMode: boolean;
	    guiMonitoring: boolean;
	    customSpeedEnabled: boolean;
	    customSpeedRPM: number;
	    ignoreDeviceOnReconnect: boolean;
	    smartControl: SmartControlConfig;
	    lightStrip: LightStripConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.legionFnQ = this.convertValues(source["legionFnQ"], LegionFnQConfig);
	        this.autoControl = source["autoControl"];
	        this.manualGearToggleHotkey = source["manualGearToggleHotkey"];
	        this.autoControlToggleHotkey = source["autoControlToggleHotkey"];
	        this.curveProfileToggleHotkey = source["curveProfileToggleHotkey"];
	        this.manualGearLevels = source["manualGearLevels"];
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
	        this.gpuSensor = source["gpuSensor"];
	        this.configPath = source["configPath"];
	        this.manualGear = source["manualGear"];
	        this.manualLevel = source["manualLevel"];
	        this.debugMode = source["debugMode"];
	        this.guiMonitoring = source["guiMonitoring"];
	        this.customSpeedEnabled = source["customSpeedEnabled"];
	        this.customSpeedRPM = source["customSpeedRPM"];
	        this.ignoreDeviceOnReconnect = source["ignoreDeviceOnReconnect"];
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
	
	    static createFrom(source: any = {}) {
	        return new TemperatureGPUDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.vendor = source["vendor"];
	        this.sensors = this.convertValues(source["sensors"], TemperatureSensor);
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
	export class BridgeTemperatureData {
	    cpuTemp: number;
	    gpuTemp: number;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
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
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
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
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
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
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
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
	

}

