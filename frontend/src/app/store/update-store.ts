'use client';

// 应用更新的下载/安装状态。
//
// 这份状态必须活在页面之外：进度弹窗原先是"关于"页面的局部状态，用户在下载途中
// 切到任何其它标签页，关于页就被卸载，进度弹窗随之消失、事件订阅也一起断掉，
// 回到关于页时只剩一个什么都没发生过的界面。放进全局 store 后，弹窗由常驻的
// AppShell 渲染，切页、切标签都不影响这次更新。

import { create } from 'zustand';
import { apiService } from '../services/api';

export type UpdateStage = 'idle' | 'downloading' | 'installing' | 'done' | 'error';

export interface UpdaterTexts {
  windowTitle: string;
  windowBody: string;
  windowRestarting: string;
}

interface UpdateState {
  stage: UpdateStage;
  percent: number;
  error: string;
  /** 当前渠道最新版本的安装包地址，空表示该版本没有可自动安装的包。 */
  installerUrl: string;
  /** Release 页面地址，失败时供用户手动下载。 */
  releaseUrl: string;
  /** 事件订阅只需要装一次，重复挂载不再重订阅。 */
  progressSubscribed: boolean;

  setRelease: (installerUrl: string, releaseUrl: string) => void;
  subscribeProgress: () => void;
  startDownloadInstall: (texts: UpdaterTexts) => Promise<string>;
  dismiss: () => void;
}

export const useUpdateStore = create<UpdateState>((set, get) => ({
  stage: 'idle',
  percent: 0,
  error: '',
  installerUrl: '',
  releaseUrl: '',
  progressSubscribed: false,

  setRelease: (installerUrl, releaseUrl) => set({ installerUrl, releaseUrl }),

  subscribeProgress: () => {
    if (get().progressSubscribed) return;
    set({ progressSubscribed: true });
    apiService.onUpdateDownloadProgress((payload) => {
      const stage = payload?.stage;
      if (stage === 'downloading') {
        set({
          stage: 'downloading',
          percent:
            typeof payload.percent === 'number' && payload.percent >= 0
              ? payload.percent
              : 0,
        });
      } else if (stage === 'installing') {
        set({ stage: 'installing', percent: 100 });
      } else if (stage === 'done') {
        set({ stage: 'done', percent: 100 });
      } else if (stage === 'error') {
        set({ stage: 'error', error: payload?.message || '' });
      }
    });
  },

  // 返回空字符串表示成功，否则是错误信息，方便调用方决定是否弹 toast。
  startDownloadInstall: async (texts) => {
    const { installerUrl } = get();
    if (!installerUrl) return '';
    set({ stage: 'downloading', percent: 0, error: '' });
    try {
      await apiService.downloadAndInstallUpdate(
        installerUrl,
        texts.windowTitle,
        texts.windowBody,
        texts.windowRestarting,
      );
      return '';
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      set({ stage: 'error', error: message });
      return message;
    }
  },

  dismiss: () => set({ stage: 'idle', percent: 0, error: '' }),
}));
