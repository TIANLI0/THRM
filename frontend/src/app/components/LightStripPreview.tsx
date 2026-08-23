import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  LIGHT_LED_COUNT,
  buildLightProgram,
  rgbToCss,
  sampleLightProgram,
  type RGB,
} from '../lib/lightProgram';

/**
 * 设备灯条的实时预览。
 *
 * 设备端是一条 6 颗灯珠的灯条，颜色由固件在关键帧之间线性插值得到。这里用
 * lightProgram.ts 复刻的同一套程序结构与插值算法驱动，因此预览显示的就是
 * 按下"应用"之后设备会呈现的画面，而不是另画的示意动画。
 */
export default function LightStripPreview({
  mode,
  speed,
  brightness,
  colors,
  className,
}: {
  mode: string;
  speed?: string;
  brightness?: number;
  colors?: Array<Partial<RGB>>;
  className?: string;
}) {
  const program = useMemo(
    () => buildLightProgram({ mode, speed, brightness, colors }),
    [mode, speed, brightness, colors],
  );

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

  return (
    <div className={className}>
      <div className="flex items-center gap-1.5 rounded-full border border-border bg-neutral-900 px-2.5 py-2 shadow-inner dark:bg-black">
        {leds.map((color, index) => {
          const lit = color.r + color.g + color.b > 0;
          return (
            <div
              key={index}
              className="h-3.5 flex-1 rounded-full transition-[background-color,box-shadow] duration-75"
              style={{
                backgroundColor: rgbToCss(color),
                boxShadow: lit ? `0 0 10px 1px ${rgbToCss(color)}` : 'inset 0 0 0 1px rgba(255,255,255,0.08)',
              }}
            />
          );
        })}
      </div>
    </div>
  );
}
