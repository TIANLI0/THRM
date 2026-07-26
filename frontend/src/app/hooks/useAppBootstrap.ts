import { useEffect } from 'react';
import { apiService } from '../services/api';
import { useAppStore } from '../store/app-store';

export function useAppBootstrap() {
  const initializeApp = useAppStore((state) => state.initializeApp);
  const startEventListeners = useAppStore((state) => state.startEventListeners);

  useEffect(() => {
    const stopListening = startEventListeners();
    return () => {
      stopListening();
    };
  }, [startEventListeners]);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  // 上报 GUI 存活时间戳。该值只用于核心的调试信息面板，因此不需要高频：
  // 连接探活已由 GUI 侧的 IPC 看护协程 + 核心侧心跳负责，不再依赖这个请求。
  // 原来是每 5 秒一次且不看窗口可见性，约合每天 1.7 万次完整 IPC 往返。
  useEffect(() => {
    let cancelled = false;
    const reportAlive = () => {
      if (cancelled || document.hidden) return;
      apiService.updateGuiResponseTime().catch(() => {
        // 后端会通过 core-service-error 事件把可见错误同步到状态层。
      });
    };

    reportAlive();
    const timer = window.setInterval(reportAlive, 30000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);
}
