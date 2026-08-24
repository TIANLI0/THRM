import React, { useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import {
  STRIP_SAMPLE_COUNT,
  averageColor,
  rgbToCss,
  sampleStripColors,
  stripGradient,
  type LightProgram,
  type RGB,
} from '../lib/lightProgram';

/**
 * 设备灯带的实时预览。
 *
 * 设备端是一整条带导光罩的灯带，看到的是一条连续的光，不是六个分开的点。
 * 所以这里沿灯条连续取样上色（sampleStripColors），位置与时间两个方向都插值，
 * 亮光在灯条上是无级滑动的。颜色本身仍由 lightProgram.ts 复刻的固件关键帧算出。
 *
 * 实物灯带约 4.5cm 长、0.4cm 宽，因此预览按同样的长宽比渲染并居中，
 * 而不是撑满整个面板宽度——撑满会把它拉成一条与实物完全不像的细长条。
 */
export default function LightStripPreview({
  program,
  className,
}: {
  program: LightProgram | null;
  className?: string;
}) {
  const off = useMemo<RGB[]>(
    () => Array.from({ length: STRIP_SAMPLE_COUNT }, () => ({ r: 0, g: 0, b: 0 })),
    [],
  );
  // 沿灯条连续取样得到的颜色，不是六颗灯珠的颜色。
  const [strip, setStrip] = useState<RGB[]>(off);
  const frameRef = useRef<number | null>(null);

  useEffect(() => {
    if (!program) {
      setStrip(off);
      return;
    }

    // 静态程序只有一帧，没必要每帧重算。
    if (program.lastKeyframe === 0) {
      setStrip(sampleStripColors(program, 0));
      return;
    }

    const start = performance.now();
    const tick = (now: number) => {
      setStrip(sampleStripColors(program, now - start));
      frameRef.current = requestAnimationFrame(tick);
    };
    frameRef.current = requestAnimationFrame(tick);
    return () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    };
  }, [program, off]);

  const glow = averageColor(strip);
  const lit = glow.r + glow.g + glow.b > 12;

  return (
    <div className={clsx('flex justify-center', className)}>
      {/* 只画灯带本身：外面再套一圈深色外壳只会变成一条黑边，实物没有这个东西。
          横向是灯珠的光晕叠加，纵向叠一层高光，看起来才像亮着的导光条。 */}
      <div
        className="aspect-[45/4] w-full max-w-[200px] rounded-full"
        style={{
          backgroundImage: [
            'linear-gradient(180deg, rgba(255,255,255,0.32) 0%, rgba(255,255,255,0.06) 42%, rgba(0,0,0,0.10) 100%)',
            stripGradient(strip),
          ].join(', '),
          boxShadow: lit
            ? `0 0 18px 3px ${rgbToCss(glow)}`
            : 'inset 0 0 0 1px color-mix(in srgb, currentColor 12%, transparent)',
        }}
      />
    </div>
  );
}
