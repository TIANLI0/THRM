// 模块级常量：在组件外定义，所有 render 共享同一个引用，避免每次重渲染都新建数组导致下游 Select 等组件 props 引用变化。
export const SMART_START_STOP_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.smartStartStop.off.label', descriptionKey: 'controlPanel.options.smartStartStop.off.description' },
  { value: 'immediate', labelKey: 'controlPanel.options.smartStartStop.immediate.label', descriptionKey: 'controlPanel.options.smartStartStop.immediate.description' },
  { value: 'delayed', labelKey: 'controlPanel.options.smartStartStop.delayed.label', descriptionKey: 'controlPanel.options.smartStartStop.delayed.description' },
];

// 温度平滑度（EMA 系数）选项。
// 数字越大平滑越强、对突发温度反应越慢；越小越灵敏但可能跟着抖。
// 后端 α = 2/(N+1)：1→即时，5→约 5 周期窗口，10→约 10 周期窗口。
export const SAMPLE_COUNT_OPTIONS = [
  { value: 1, labelKey: 'controlPanel.options.sampleCount.1' },
  { value: 2, labelKey: 'controlPanel.options.sampleCount.2' },
  { value: 3, labelKey: 'controlPanel.options.sampleCount.3' },
  { value: 5, labelKey: 'controlPanel.options.sampleCount.5' },
  { value: 10, labelKey: 'controlPanel.options.sampleCount.10' },
];

export const TEMP_SOURCE_OPTIONS = [
  { value: 'max', labelKey: 'controlPanel.options.tempSource.max' },
  { value: 'cpu', labelKey: 'controlPanel.options.tempSource.cpu' },
  { value: 'gpu', labelKey: 'controlPanel.options.tempSource.gpu' },
];

// 内置基础主题选项。自定义主题（如 THRM）改为运行时从安装目录动态发现并追加。
export const THEME_MODE_OPTIONS = [
  { value: 'light', labelKey: 'controlPanel.options.themeMode.light' },
  { value: 'dark', labelKey: 'controlPanel.options.themeMode.dark' },
  { value: 'system', labelKey: 'controlPanel.options.themeMode.system' },
];
