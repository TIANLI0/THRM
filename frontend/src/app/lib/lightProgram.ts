/**
 * 设备灯条程序的主机侧复刻。
 *
 * 这份实现对照 CH591 固件反编译结果写成，用于在应用内预览"设备实际会显示什么"，
 * 而不是另画一套示意动画。三处关键结构都来自固件本身：
 *
 * 1. 上传缓冲区布局（firmware `initialize_status_reporting`，写入 0x200049c8）：
 *
 *        [0]=flags0 [1]=flags1 [2]=firstKeyframe [3]=lastKeyframe
 *        [4]=transitionTicks [5]=brightness [6..]=颜色区
 *
 *    颜色区是灯珠主序，某一颜色的偏移为 `6 + led*30 + keyframe*3`
 *    （固件 `FUN_ram_00007fa8` 的重排循环用的就是 led 步进 0x1e、关键帧步进 3）。
 *
 * 2. 灯条是 6 颗灯珠。固件帧缓冲区 0x20004be0..0x20004bf2 正好 18 字节 = 6 × RGB，
 *    移位输出函数按 G、R、B 的顺序发送（WS2812 系列的线序），缓冲区里仍是 RGB。
 *
 * 3. 亮度与插值（固件 `FUN_ram_000080b2` / `FUN_ram_00008180`）：
 *    装载关键帧时先做 `value * brightness / 100`，然后在当前帧与下一帧之间线性插值，
 *    一次过渡共 `transitionTicks * 10` 个子步，走完后切到下一关键帧并回环。
 *    因此亮度只能由固件缩放一次——主机预先缩放会让实际亮度变成平方。
 */

export const LIGHT_LED_COUNT = 6;
export const LIGHT_KEYFRAME_COUNT = 10;

/** 与 internal/device/rgb.go 的 parseLightSpeed 一致。 */
export const LIGHT_SPEED_FAST = 0x05;
export const LIGHT_SPEED_MEDIUM = 0x0a;
export const LIGHT_SPEED_SLOW = 0x0f;

/**
 * 固件一个子步对应的真实时长。固件的 LED 定时器周期无法从 stripped 镜像中读出，
 * 这里按 10ms/子步标定，使中速（transitionTicks=10）的一次过渡约为 1 秒，
 * 与实机观感一致。它只影响预览速度，不影响下发的任何字节。
 */
export const LIGHT_SUBSTEP_MS = 10;

export interface RGB {
  r: number;
  g: number;
  b: number;
}

export interface LightProgram {
  /** 关键帧上界（含）。0 表示静态单帧。 */
  lastKeyframe: number;
  /** 过渡节拍，一次关键帧过渡共 transitionTicks * 10 个子步。 */
  transitionTicks: number;
  /** 0..100，由固件在装载关键帧时缩放。 */
  brightness: number;
  /** colors[led][keyframe]，未缩放。 */
  colors: RGB[][];
}

const BLACK: RGB = { r: 0, g: 0, b: 0 };

function emptyColors(): RGB[][] {
  return Array.from({ length: LIGHT_LED_COUNT }, () =>
    Array.from({ length: LIGHT_KEYFRAME_COUNT }, () => ({ ...BLACK })),
  );
}

function clampByte(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(255, Math.round(value)));
}

function normalizeColor(color: Partial<RGB> | undefined): RGB {
  return { r: clampByte(color?.r ?? 0), g: clampByte(color?.g ?? 0), b: clampByte(color?.b ?? 0) };
}

export function parseLightSpeed(speed: string): number {
  switch (speed) {
    case 'fast':
      return LIGHT_SPEED_FAST;
    case 'slow':
      return LIGHT_SPEED_SLOW;
    default:
      return LIGHT_SPEED_MEDIUM;
  }
}

function newProgram(lastKeyframe: number, transitionTicks: number, brightness: number): LightProgram {
  return { lastKeyframe, transitionTicks, brightness, colors: emptyColors() };
}

function ensureMinColors(colors: RGB[], min: number): RGB[] {
  const defaults: RGB[] = [
    { r: 255, g: 0, b: 0 },
    { r: 0, g: 255, b: 0 },
    { r: 0, g: 128, b: 255 },
  ];
  const result = colors.slice();
  while (result.length < min) result.push(defaults[result.length % defaults.length]);
  return result;
}

/**
 * 按灯效模式构造程序，逐条对应 internal/device/rgb.go 里的 setLightXxxLocked。
 * 返回 null 表示灯条关闭。smart_temp 走固件原生预设，不是上传程序，因此也返回 null。
 */
export function buildLightProgram(config: {
  mode: string;
  speed?: string;
  brightness?: number;
  colors?: Array<Partial<RGB>>;
}): LightProgram | null {
  const brightness = Math.max(0, Math.min(100, Math.round(config.brightness ?? 100)));
  const speed = parseLightSpeed(config.speed || 'medium');
  const colors = (config.colors || []).map(normalizeColor);

  switch (config.mode) {
    case 'static_single': {
      const color = colors[0] ?? { r: 255, g: 255, b: 255 };
      const program = newProgram(0, LIGHT_SPEED_MEDIUM, brightness);
      for (let led = 0; led < LIGHT_LED_COUNT; led++) program.colors[led][0] = color;
      return program;
    }
    case 'static_multi': {
      const base: RGB[] = [
        { r: 255, g: 0, b: 0 },
        { r: 0, g: 255, b: 0 },
        { r: 0, g: 128, b: 255 },
      ];
      for (let i = 0; i < base.length && i < colors.length; i++) base[i] = colors[i];
      const program = newProgram(0, LIGHT_SPEED_MEDIUM, brightness);
      for (let led = 0; led < LIGHT_LED_COUNT; led++) program.colors[led][0] = base[led % base.length];
      return program;
    }
    case 'rotation': {
      const palette = ensureMinColors(colors, 1).slice(0, LIGHT_LED_COUNT);
      const program = newProgram(LIGHT_LED_COUNT - 1, speed, brightness);
      // 固件侧只写入 colorIndex < 颜色数的位置，其余关键帧保持黑色，
      // 于是颜色不足 6 个时会出现"跑马灯留白"，这是设备的真实表现。
      for (let led = 0; led < LIGHT_LED_COUNT; led++) {
        for (let keyframe = 0; keyframe < LIGHT_LED_COUNT; keyframe++) {
          const colorIndex = (led + keyframe) % LIGHT_LED_COUNT;
          if (colorIndex < palette.length) program.colors[led][keyframe] = palette[colorIndex];
        }
      }
      return program;
    }
    case 'flowing': {
      const palette: RGB[] = [
        { r: 255, g: 0, b: 0 },
        { r: 255, g: 255, b: 0 },
        { r: 0, g: 255, b: 0 },
        { r: 0, g: 255, b: 255 },
        { r: 0, g: 0, b: 255 },
        { r: 255, g: 0, b: 255 },
      ];
      const program = newProgram(palette.length - 1, speed, brightness);
      for (let led = 0; led < LIGHT_LED_COUNT; led++) {
        for (let keyframe = 0; keyframe < palette.length; keyframe++) {
          program.colors[led][keyframe] = palette[(led + keyframe) % palette.length];
        }
      }
      return program;
    }
    case 'breathing': {
      const palette = ensureMinColors(colors, 1).slice(0, 5);
      const program = newProgram(palette.length * 2 - 1, speed, brightness);
      // 奇数关键帧留黑，固件插值因此把每种颜色淡出到黑再进入下一种。
      for (let led = 0; led < LIGHT_LED_COUNT; led++) {
        palette.forEach((color, i) => {
          program.colors[led][i * 2] = color;
        });
      }
      return program;
    }
    default:
      return null;
  }
}

/** 固件装载关键帧时的亮度缩放：value * brightness / 100，整数截断。 */
function scale(value: number, brightness: number): number {
  return Math.trunc((value * brightness) / 100);
}

/**
 * 复刻固件 FUN_ram_00008180 的插值：在关键帧 k 与 k+1（回环）之间线性过渡，
 * 一次过渡 transitionTicks * 10 个子步。elapsedMs 为动画已播放的时长。
 */
export function sampleLightProgram(program: LightProgram, elapsedMs: number): RGB[] {
  const keyframeCount = Math.max(1, program.lastKeyframe + 1);
  const output: RGB[] = [];

  if (keyframeCount === 1) {
    for (let led = 0; led < LIGHT_LED_COUNT; led++) {
      const color = program.colors[led][0];
      output.push({
        r: scale(color.r, program.brightness),
        g: scale(color.g, program.brightness),
        b: scale(color.b, program.brightness),
      });
    }
    return output;
  }

  const substeps = Math.max(1, program.transitionTicks * 10);
  const transitionMs = substeps * LIGHT_SUBSTEP_MS;
  const cycleMs = transitionMs * keyframeCount;
  const position = ((elapsedMs % cycleMs) + cycleMs) % cycleMs;
  const current = Math.floor(position / transitionMs);
  const next = (current + 1) % keyframeCount;
  // 固件按整数子步推进，这里同样量化，避免预览比设备更"顺滑"。
  const step = Math.floor((position - current * transitionMs) / LIGHT_SUBSTEP_MS);

  for (let led = 0; led < LIGHT_LED_COUNT; led++) {
    const from = program.colors[led][current];
    const to = program.colors[led][next];
    const mix = (a: number, b: number) => {
      const start = scale(a, program.brightness);
      const end = scale(b, program.brightness);
      return clampByte(Math.trunc(((end - start) * step) / substeps) + start);
    };
    output.push({ r: mix(from.r, to.r), g: mix(from.g, to.g), b: mix(from.b, to.b) });
  }
  return output;
}

export function rgbToCss(color: RGB): string {
  return `rgb(${color.r}, ${color.g}, ${color.b})`;
}

/**
 * 灯条是一整条、带导光罩的灯带，不是六个分开的点：相邻灯珠的光会互相融合。
 * 因此渲染成一条连续渐变，色标落在各灯珠的中心位置 (i + 0.5) / 6，
 * 两端补上首尾灯珠的颜色，避免边缘出现突兀的截断。
 */
export function stripGradient(colors: RGB[]): string {
  if (colors.length === 0) return 'rgb(0,0,0)';
  const stops: string[] = [`${rgbToCss(colors[0])} 0%`];
  colors.forEach((color, index) => {
    const center = ((index + 0.5) / colors.length) * 100;
    stops.push(`${rgbToCss(color)} ${center.toFixed(2)}%`);
  });
  stops.push(`${rgbToCss(colors[colors.length - 1])} 100%`);
  return `linear-gradient(90deg, ${stops.join(', ')})`;
}

/** 用于外发光的平均色。 */
export function averageColor(colors: RGB[]): RGB {
  if (colors.length === 0) return { ...BLACK };
  const sum = colors.reduce(
    (acc, c) => ({ r: acc.r + c.r, g: acc.g + c.g, b: acc.b + c.b }),
    { r: 0, g: 0, b: 0 },
  );
  return {
    r: Math.round(sum.r / colors.length),
    g: Math.round(sum.g / colors.length),
    b: Math.round(sum.b / colors.length),
  };
}

/* ── 智能温控的原生预设 ──

固件的 0x44 预设生成器无法从镜像中恢复：它挂在 RGB 控制器虚表的 +0x10 槽上，
而这张虚表位于已初始化数据段（复位时拷进 RAM），反编译器看不到谁填的它——
`FUN_ram_00008480` 等同槽位的函数在整个反编译产物里都没有具名引用。

因此下面的预览是**示意**，不是逐字节复刻：颜色取自实机观察（预设 1/2/3 依次为
绿、黄、红），动作用固件同一套关键帧插值做成呼吸。预设 4/5 固件接受但外观没有
记录，这里不编造，返回 null。界面必须把这一点标注清楚。
*/

export const SMART_TEMP_PRESET_COLORS: Record<number, RGB | null> = {
  1: { r: 34, g: 197, b: 94 },
  2: { r: 234, g: 179, b: 8 },
  3: { r: 239, g: 68, b: 68 },
  4: null,
  5: null,
};

/** 该预设的外观是否有实机依据。false 表示界面应显示"未知"而不是猜一个颜色。 */
export function isSmartTempPresetKnown(preset: number): boolean {
  return SMART_TEMP_PRESET_COLORS[preset] != null;
}

/**
 * 构造智能温控预设的示意动画：整条灯带同色呼吸。
 * 走的是与自定义灯效相同的关键帧插值，只是关键帧由这里合成。
 */
export function buildSmartTempPreviewProgram(preset: number, brightness = 100): LightProgram | null {
  const color = SMART_TEMP_PRESET_COLORS[preset];
  if (!color) return null;

  const dim: RGB = {
    r: Math.round(color.r * 0.22),
    g: Math.round(color.g * 0.22),
    b: Math.round(color.b * 0.22),
  };
  const program = newProgram(1, LIGHT_SPEED_SLOW, Math.max(0, Math.min(100, Math.round(brightness))));
  for (let led = 0; led < LIGHT_LED_COUNT; led++) {
    program.colors[led][0] = color;
    program.colors[led][1] = dim;
  }
  return program;
}
