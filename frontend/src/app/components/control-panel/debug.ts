type ParsedGearTable = { type?: string; table?: Array<{ gear?: number; label?: string; rpm?: number }> };

export function renderDebugFrameSummary(frame: { decoded?: string; parsed?: unknown; command?: string; payloadHex?: string }) {
  const parsed = frame.parsed as ParsedGearTable | null | undefined;
  if (parsed?.type === 'gearRpmTable' && Array.isArray(parsed.table)) {
    return parsed.table
      .map((item) => `${item.label || item.gear}: ${item.rpm} RPM`)
      .join(' | ');
  }
  return frame.decoded || `${frame.command || '--'} ${frame.payloadHex || ''}`.trim();
}

// 高危调试命令：直接读写固件底层/调试寄存器，误用可能导致设备异常甚至变砖，需在发送前红色提醒。
export const DANGEROUS_DEBUG_COMMANDS = new Set<number>([
  0x03, // initializes controller and can select fixed gear 1
  0x05, // clears only the firmware initialization latch
  0x06, // resets RPM slots plus runtime/startup/LED state
  0xed,
  0xee,
  0xf0,
  0xf1,
  0xf2,
]);
export const FIRMWARE_MAINTENANCE_COMMANDS = new Set<number>([0x03, 0x05, 0x06]);

// 从用户输入中解析出命令字节：兼容 "27" 与 "5A A5 27 02 29"（带帧头时命令为第三字节）。
export function parseDebugCommandByte(input: string): number | null {
  const bytes = input
    .trim()
    .split(/[^0-9a-fA-F]+/)
    .filter(Boolean)
    .map((h) => Number.parseInt(h, 16));
  if (bytes.length === 0 || bytes.some((b) => Number.isNaN(b))) return null;
  if (bytes.length >= 3 && bytes[0] === 0x5a && bytes[1] === 0xa5) return bytes[2];
  return bytes[0];
}
