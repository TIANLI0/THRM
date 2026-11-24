'use client';

import React, { useState, useEffect } from 'react';

/**
 * 噪音测试与散热收益两个对话框只在打开时才有内容，代码却不小（含各自的图表与算法）。
 * 不随曲线页一起加载：曲线页挂载后的空闲时间里再拉取（用户抢先点了按钮则立即拉取），
 * 加载完成后照常常驻挂载，open 切换、开关动画与内部状态都和直接引入时一样。
 *
 * 不用 React.lazy：首次打开时一旦触发 Suspense，React 19 会把内容的显示节流到
 * fallback 出现 300ms 之后，对话框会明显迟一拍。
 */
function createDeferredDialog<P extends { open: boolean }>(load: () => Promise<{ default: React.ComponentType<P> }>) {
  let loaded: React.ComponentType<P> | null = null;
  let pending: Promise<React.ComponentType<P>> | null = null;
  const ensureLoaded = () => {
    pending ??= load().then(
      (mod) => (loaded = mod.default),
      (error) => {
        pending = null;
        throw error;
      },
    );
    return pending;
  };

  return function DeferredDialog(props: P) {
    const [Component, setComponent] = useState<React.ComponentType<P> | null>(() => loaded);

    useEffect(() => {
      if (Component) return;
      let cancelled = false;
      const apply = () => {
        ensureLoaded().then(
          (component) => {
            if (!cancelled) setComponent(() => component);
          },
          () => {},
        );
      };
      if (props.open) {
        apply();
        return () => {
          cancelled = true;
        };
      }
      if (typeof window.requestIdleCallback === 'function') {
        const handle = window.requestIdleCallback(apply, { timeout: 2000 });
        return () => {
          cancelled = true;
          window.cancelIdleCallback(handle);
        };
      }
      const handle = window.setTimeout(apply, 200);
      return () => {
        cancelled = true;
        window.clearTimeout(handle);
      };
    }, [Component, props.open]);

    return Component ? <Component {...props} /> : null;
  };
}

export const NoiseTest = createDeferredDialog(() => import('../NoiseTest'));
export const CoolingBenefit = createDeferredDialog(() => import('../CoolingBenefit'));
