// 散热收益扫描测试。
//
// 与噪音测试同一套骨架（锁定转速 → 等稳定 → 采样），但等待的东西完全不同：
// 声学在 1~2 秒内就稳了，热惯性要一两分钟。所以这里不能沿用固定的短等待，
// 必须真的盯着温度看它是否还在漂——提前采样得到的是过渡态，画出来的曲线
// 会把"还没热起来"当成"散热器有效"。
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';

export const BENEFIT_MIN_RPM = 1000;
export const BENEFIT_MAX_RPM = 4000;
export const BENEFIT_STEP_RPM = 600;

// 热稳定判据：温度在该窗口内的极差小于阈值即认为稳了。
const SETTLE_POLL_MS = 3000;
const SETTLE_WINDOW = 6;          // 约 18 秒的滚动窗口
const SETTLE_TEMP_RANGE_C = 1.5;
const SETTLE_MIN_MS = 30_000;     // 至少等这么久，避免一进来就"恰好"稳
const SETTLE_MAX_MS = 150_000;    // 等不稳也要走，否则一次测试会没完没了
const SAMPLE_MS = 15_000;
const SAMPLE_INTERVAL_MS = 1500;

// 开始测试前要求的最低负载。低于它所有转速都一样凉，测出来没有区分度。
export const MIN_LOAD_WATTS = 20;

export interface BenefitStepProgress {
  index: number;
  total: number;
  rpm: number;
  phase: 'settling' | 'sampling';
  elapsedMs: number;
  /** 稳定阶段的实时温度极差，让用户看见"它在等什么"。 */
  tempRangeC: number;
}

export function buildBenefitSteps(): number[] {
  const steps: number[] = [];
  for (let rpm = BENEFIT_MIN_RPM; rpm < BENEFIT_MAX_RPM; rpm += BENEFIT_STEP_RPM) {
    steps.push(rpm);
  }
  steps.push(BENEFIT_MAX_RPM);
  return steps;
}

function createAbortError(): Error {
  return new DOMException('cooling benefit test aborted', 'AbortError');
}

export function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError';
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) { reject(createAbortError()); return; }
    const timer = setTimeout(() => { cleanup(); resolve(); }, ms);
    const onAbort = () => { cleanup(); reject(createAbortError()); };
    const cleanup = () => { clearTimeout(timer); signal?.removeEventListener('abort', onAbort); };
    signal?.addEventListener('abort', onAbort);
  });
}

function hottest(temp: types.TemperatureData): number {
  return Math.max(temp.cpuTemp || 0, temp.gpuTemp || 0);
}

function totalPower(temp: types.TemperatureData): number {
  return (temp.cpuPower || 0) + (temp.gpuPower || 0);
}

function range(values: number[]): number {
  if (values.length === 0) return 0;
  return Math.max(...values) - Math.min(...values);
}

function mean(values: number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((sum, v) => sum + v, 0) / values.length;
}

/** 读一次温度，失败返回 null（桥接抖动不该中断整场测试）。 */
async function readTemperature(): Promise<types.TemperatureData | null> {
  try {
    return await apiService.getTemperature();
  } catch {
    return null;
  }
}

export interface LoadCheck {
  ok: boolean;
  watts: number;
  hasPowerReadings: boolean;
}

// 开测前确认机器确实在干活。没有这一步，用户很容易在待机状态下跑完十分钟
// 然后得到一张平坦的图，还以为是散热器没用。
export async function checkLoad(): Promise<LoadCheck> {
  const temp = await readTemperature();
  if (!temp) return { ok: false, watts: 0, hasPowerReadings: false };
  const watts = totalPower(temp);
  return { ok: watts >= MIN_LOAD_WATTS, watts, hasPowerReadings: watts > 0 };
}

type SensorGroup = 'cpu' | 'gpu';

interface SensorAccumulator {
  name: string;
  group: SensorGroup;
  sum: number;
  count: number;
}

/**
 * 把一帧温度里的全部具名传感器累加进 acc。
 *
 * GPU 侧优先走 gpuDevices：顶层的 gpuSensors 只是当前选中那块显卡的传感器，
 * 而报告要回答"哪个零部件受益最大"，就不能漏掉另一块 GPU。两者会指向同一批
 * 物理传感器，所以有 gpuDevices 时不再重复采顶层列表。
 */
function accumulateSensors(acc: Map<string, SensorAccumulator>, temp: types.TemperatureData): void {
  const push = (key: string, name: string, group: SensorGroup, value: number) => {
    if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return;
    const existing = acc.get(key);
    if (existing) {
      existing.sum += value;
      existing.count += 1;
      return;
    }
    acc.set(key, { name, group, sum: value, count: 1 });
  };

  for (const sensor of temp.cpuSensors ?? []) {
    if (!sensor?.key) continue;
    push(`cpu/${sensor.key}`, sensor.name || sensor.key, 'cpu', sensor.value);
  }

  const devices = temp.gpuDevices ?? [];
  if (devices.length > 0) {
    for (const device of devices) {
      for (const sensor of device?.sensors ?? []) {
        if (!sensor?.key) continue;
        // 多显卡时同名传感器（Hot Spot 之类）会撞车，键和显示名都带上设备标识。
        const label = devices.length > 1 && device.name ? `${device.name} · ${sensor.name || sensor.key}` : (sensor.name || sensor.key);
        push(`gpu/${device.key}/${sensor.key}`, label, 'gpu', sensor.value);
      }
    }
    return;
  }
  for (const sensor of temp.gpuSensors ?? []) {
    if (!sensor?.key) continue;
    push(`gpu/${sensor.key}`, sensor.name || sensor.key, 'gpu', sensor.value);
  }
}

function finalizeSensors(acc: Map<string, SensorAccumulator>): types.CoolingSensorReading[] {
  const out: types.CoolingSensorReading[] = [];
  for (const [key, entry] of acc) {
    if (entry.count === 0) continue;
    out.push({
      key,
      name: entry.name,
      group: entry.group,
      value: Math.round((entry.sum / entry.count) * 10) / 10,
    });
  }
  return out;
}

// waitForThermalSettle 盯着控温温度直到它不再漂，或者到达超时。
// 返回是否真的稳住了——没稳住的档位会在报告里标注，而不是假装它稳了。
async function waitForThermalSettle(
  onTick: (elapsedMs: number, tempRangeC: number) => void,
  signal?: AbortSignal,
): Promise<boolean> {
  const start = Date.now();
  const window: number[] = [];

  while (Date.now() - start < SETTLE_MAX_MS) {
    await sleep(SETTLE_POLL_MS, signal);
    const temp = await readTemperature();
    if (temp) {
      const value = hottest(temp);
      if (value > 0) {
        window.push(value);
        if (window.length > SETTLE_WINDOW) window.shift();
      }
    }
    const spread = window.length >= SETTLE_WINDOW ? range(window) : Number.POSITIVE_INFINITY;
    const elapsed = Date.now() - start;
    onTick(elapsed, Number.isFinite(spread) ? spread : 0);

    if (elapsed >= SETTLE_MIN_MS && window.length >= SETTLE_WINDOW && spread <= SETTLE_TEMP_RANGE_C) {
      return true;
    }
  }
  return false;
}

// sampleStep 在已经热稳定的前提下采集一个档位的平均读数。
// 同时记录采样窗口内的极差：它是这一档"到底稳没稳"的证据，
// 后端的可信度判定就靠它，不能只把平均值报上去。
async function sampleStep(targetRPM: number, settled: boolean, signal?: AbortSignal): Promise<types.CoolingBenefitStep> {
  const cpuTemps: number[] = [];
  const gpuTemps: number[] = [];
  const cpuPowers: number[] = [];
  const gpuPowers: number[] = [];
  const controlTemps: number[] = [];
  const totals: number[] = [];
  const laptopCPU: number[] = [];
  const laptopGPU: number[] = [];
  const actualRPMs: number[] = [];
  // 传感器同样取整段采样窗口的均值：单帧读数会被瞬时波动带偏，
  // 而报告里的"降了几度"是要给用户看结论的。
  const sensorAcc = new Map<string, SensorAccumulator>();

  const count = Math.max(3, Math.round(SAMPLE_MS / SAMPLE_INTERVAL_MS));
  for (let i = 0; i < count; i++) {
    await sleep(SAMPLE_INTERVAL_MS, signal);
    const temp = await readTemperature();
    if (!temp) continue;

    if (temp.cpuTemp > 0) cpuTemps.push(temp.cpuTemp);
    if (temp.gpuTemp > 0) gpuTemps.push(temp.gpuTemp);
    if (temp.cpuPower > 0) cpuPowers.push(temp.cpuPower);
    if (temp.gpuPower > 0) gpuPowers.push(temp.gpuPower);
    if (temp.cpuFanRpm > 0) laptopCPU.push(temp.cpuFanRpm);
    if (temp.gpuFanRpm > 0) laptopGPU.push(temp.gpuFanRpm);
    controlTemps.push(hottest(temp));
    totals.push(totalPower(temp));

    accumulateSensors(sensorAcc, temp);

    try {
      const fanData = await apiService.getCurrentFanData();
      if (fanData && fanData.currentRpm > 0) actualRPMs.push(fanData.currentRpm);
    } catch {
      /* 读不到实际转速时留空，后端只把它用于"够不够得到目标转速"的告警 */
    }
  }

  return {
    targetRpm: targetRPM,
    actualRpm: Math.round(mean(actualRPMs)),
    cpuTemp: Math.round(mean(cpuTemps) * 10) / 10,
    gpuTemp: Math.round(mean(gpuTemps) * 10) / 10,
    cpuPower: Math.round(mean(cpuPowers) * 10) / 10,
    gpuPower: Math.round(mean(gpuPowers) * 10) / 10,
    laptopCpuFanRpm: Math.round(mean(laptopCPU)),
    laptopGpuFanRpm: Math.round(mean(laptopGPU)),
    sensors: finalizeSensors(sensorAcc),
    samples: controlTemps.length,
    // 没等稳的档位人为放大极差上报，让后端的 notSettled 告警一定能触发——
    // 采样窗口很短，一个仍在缓慢爬升的档位在窗口内看着可能相当平稳。
    tempRange: Math.round((settled ? range(controlTemps) : Math.max(range(controlTemps), 99)) * 10) / 10,
    powerRange: Math.round(range(totals) * 10) / 10,
  } as types.CoolingBenefitStep;
}

/**
 * 执行完整扫描。调用方负责事先 captureFanState、事后 restoreFanState
 * （复用噪音测试的那一对，两者对风扇的接管方式完全相同）。
 */
export async function runBenefitSweep(
  onProgress: (progress: BenefitStepProgress, steps: types.CoolingBenefitStep[]) => void,
  signal?: AbortSignal,
): Promise<types.CoolingBenefitStep[]> {
  const rpmSteps = buildBenefitSteps();
  const results: types.CoolingBenefitStep[] = [];

  for (let i = 0; i < rpmSteps.length; i++) {
    if (signal?.aborted) throw createAbortError();
    const rpm = rpmSteps[i];

    await apiService.setCustomSpeed(true, rpm);
    onProgress({ index: i, total: rpmSteps.length, rpm, phase: 'settling', elapsedMs: 0, tempRangeC: 0 }, results);
    const settled = await waitForThermalSettle(
      (elapsedMs, tempRangeC) => onProgress({ index: i, total: rpmSteps.length, rpm, phase: 'settling', elapsedMs, tempRangeC }, results),
      signal,
    );

    onProgress({ index: i, total: rpmSteps.length, rpm, phase: 'sampling', elapsedMs: 0, tempRangeC: 0 }, results);
    results.push(await sampleStep(rpm, settled, signal));
    onProgress({ index: i, total: rpmSteps.length, rpm, phase: 'sampling', elapsedMs: SAMPLE_MS, tempRangeC: 0 }, results);
  }

  return results;
}

/** 一次完整扫描的预计耗时，用于在开测前如实告知用户。 */
export function estimatedDurationMs(): number {
  return buildBenefitSteps().length * (SETTLE_MIN_MS + SAMPLE_MS);
}
