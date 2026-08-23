import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  LIGHT_LED_COUNT,
  averageColor,
  rgbToCss,
  sampleLightProgram,
  stripGradient,
  type LightProgram,
  type RGB,
} from '../lib/lightProgram';

/**
 * 设备灯带的实时预览。
 *
 * 设备端是一整条带导光罩的灯带（6 颗灯珠串在一起），相邻灯珠的光互相融合，
 * 所以这里渲染成一条连续渐变，而不是六个分开的点。颜色由 lightProgram.ts
 * 复刻的固件关键帧插值逐帧算出。
 */
export default function LightStripPreview({
  program,
  className,
}: {
  program: LightProgram | null;
  className?: string;
}) {
  const off = useMemo<RGB[]>(
    () => Array.from({ length: LIGHT_LED_COUNT }, () => ({ r: 0, g: 0, b: 0 })),
    [],
  );
  const [leds, setLeds] = useState<RGB[]>(off);
  const frameRef = useRef<number | null>(null);

  useEffect(() => {
    if (!program) {
      setLeds(off);
      return;
    }

    // 静态程序只有一帧，没必要每帧重算。
    if (program.lastKeyframe === 0) {
      setLeds(sampleLightProgram(program, 0));
      return;
    }

    const start = performance.now();
    const tick = (now: number) => {
      setLeds(sampleLightProgram(program, now - start));
      frameRef.current = requestAnimationFrame(tick);
    };
    frameRef.current = requestAnimationFrame(tick);
    return () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    };
  }, [program, off]);

  const glow = averageColor(leds);
  const lit = glow.r + glow.g + glow.b > 12;

  return (
    <div className={className}>
      <div className="rounded-full border border-border bg-neutral-900 p-1.5 shadow-inner dark:bg-black">
        <div
          className="h-4 w-full rounded-full"
          style={{
            background: stripGradient(leds),
            boxShadow: lit
              ? `0 0 16px 2px ${rgbToCss(glow)}, inset 0 1px 1px rgba(255,255,255,0.25)`
              : 'inset 0 0 0 1px rgba(255,255,255,0.08)',
          }}
        />
      </div>
    </div>
  );
}
