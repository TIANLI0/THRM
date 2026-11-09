'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { TFunction } from 'i18next';
import { Cpu, Download, FileUp, HardDriveDownload, Loader2, RefreshCw, ShieldCheck, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import {
  apiService,
  type FirmwareUpdateStatus,
  type FirmwareInfo,
  type FirmwareFlashEvent,
} from '../services/api';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  t: TFunction;
}

export function FirmwareUpdatePanel({ open, onOpenChange, t }: Props) {
  const [status, setStatus] = useState<FirmwareUpdateStatus | null>(null);
  const [list, setList] = useState<FirmwareInfo[]>([]);
  const [selected, setSelected] = useState<FirmwareInfo | null>(null);
  const [localPath, setLocalPath] = useState('');
  const [checking, setChecking] = useState(false);
  const [flashing, setFlashing] = useState(false);
  const [progress, setProgress] = useState<{ phase: string; pct: number } | null>(null);
  const [logLines, setLogLines] = useState<string[]>([]);
  const logRef = useRef<HTMLDivElement>(null);

  const refreshStatus = useCallback(async () => {
    try {
      setStatus(await apiService.getFirmwareUpdateStatus());
    } catch { /* noop */ }
  }, []);

  // 打开时拉状态，并每 2 秒轮询（用于检测进入下载模式）。
  // 窗口隐藏时跳过：没人看着，重新可见后下一次轮询就会补上。
  useEffect(() => {
    if (!open) return;
    refreshStatus();
    const id = setInterval(() => {
      if (!document.hidden) refreshStatus();
    }, 2000);
    return () => clearInterval(id);
  }, [open, refreshStatus]);

  // 订阅刷写进度/日志/完成事件。
  useEffect(() => {
    if (!open) return;
    const off = apiService.onFirmwareUpdate((ev: FirmwareFlashEvent) => {
      if (ev.kind === 'log' && ev.message) {
        setLogLines((prev) => [...prev, ev.message!]);
      } else if (ev.kind === 'progress') {
        const total = ev.total || 1;
        setProgress({ phase: ev.phase || '', pct: Math.round(((ev.done || 0) * 100) / total) });
      } else if (ev.kind === 'done') {
        setFlashing(false);
        setProgress(null);
        if (ev.success) {
          toast.success(t('firmwareUpdate.flashSuccess'));
          setLogLines((prev) => [...prev, '✓ ' + (ev.message || '')]);
        } else {
          toast.error(ev.message || t('firmwareUpdate.flashFailed'));
          setLogLines((prev) => [...prev, '✗ ' + (ev.message || '')]);
        }
        refreshStatus();
      }
    });
    return off;
  }, [open, refreshStatus, t]);

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight });
  }, [logLines]);

  const checkUpdates = useCallback(async () => {
    setChecking(true);
    try {
      const items = await apiService.listFirmwareUpdates();
      setList(items);
      if (items.length > 0) setSelected(items[0]);
      if (items.length === 0) toast.info(t('firmwareUpdate.noFirmware'));
    } catch (e) {
      toast.error(String((e as Error)?.message ?? e));
    } finally {
      setChecking(false);
    }
  }, [t]);

  const pickLocal = useCallback(async () => {
    try {
      const path = await apiService.selectFirmwareFile();
      if (path) {
        setLocalPath(path);
        setSelected({ source: 'local', version: '', name: path, info: '', date: '', url: '' });
      }
    } catch { /* noop */ }
  }, []);

  const startFlash = useCallback(async () => {
    if (!selected) return;
    setFlashing(true);
    setLogLines([]);
    setProgress(null);
    try {
      await apiService.flashFirmware({
        source: selected.source,
        url: selected.url,
        localPath: selected.source === 'local' ? localPath : '',
        version: selected.version,
      });
    } catch (e) {
      setFlashing(false);
      toast.error(String((e as Error)?.message ?? e));
    }
  }, [selected, localPath]);

  const installDriver = useCallback(async () => {
    try {
      await apiService.installFirmwareDriver();
      toast.success(t('firmwareUpdate.driverInstalled'));
      refreshStatus();
    } catch (e) {
      toast.error(String((e as Error)?.message ?? e));
    }
  }, [refreshStatus, t]);

  const uninstallDriver = useCallback(async () => {
    try {
      await apiService.uninstallFirmwareDriver();
      toast.success(t('firmwareUpdate.driverUninstalled'));
      refreshStatus();
    } catch (e) {
      toast.error(String((e as Error)?.message ?? e));
    }
  }, [refreshStatus, t]);

  const inISP = status?.devicePresentIsp ?? false;
  const driverOK = status?.driverInstalled ?? false;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!flashing) onOpenChange(o); }}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <HardDriveDownload className="h-5 w-5" />
            {t('firmwareUpdate.title')}
          </DialogTitle>
          <DialogDescription>{t('firmwareUpdate.subtitle')}</DialogDescription>
        </DialogHeader>

        {!status?.supported ? (
          <div className="py-6 text-sm text-muted-foreground">{t('firmwareUpdate.unsupported')}</div>
        ) : (
          <div className="space-y-4">
            {/* 设备 + 下载模式状态 */}
            <div className="rounded-lg border border-border/60 p-3 text-sm">
              <div className="flex items-center gap-2">
                <Cpu className="h-4 w-4 text-primary" />
                <span className="font-medium">{status?.modelName || t('firmwareUpdate.unknownDevice')}</span>
                <span className="text-xs text-muted-foreground">{status?.deviceCode}</span>
              </div>
              {status?.currentVersion && (
                <div className="mt-1 text-xs text-muted-foreground">
                  {t('firmwareUpdate.currentFirmware')}: <span className="font-mono">{status.currentVersion}</span>
                  {(status.thrmVersion ?? 0) > 0 && (
                    <span className="ml-1 rounded bg-primary/15 px-1.5 py-0.5 text-primary">THRM v{status.thrmVersion}</span>
                  )}
                </div>
              )}
              <div className="mt-2 flex flex-wrap gap-2 text-xs">
                <Badge ok={inISP}>{inISP ? t('firmwareUpdate.ispReady') : t('firmwareUpdate.ispWaiting')}</Badge>
                {inISP && <Badge ok={driverOK}>{driverOK ? t('firmwareUpdate.driverOk') : t('firmwareUpdate.driverMissing')}</Badge>}
              </div>
              {!inISP && (
                <p className="mt-2 text-xs text-amber-500">{t('firmwareUpdate.enterDownloadMode')}</p>
              )}
              {inISP && !driverOK && (
                <div className="mt-2 flex gap-2">
                  <Button size="sm" variant="outline" onClick={installDriver}>
                    <ShieldCheck className="mr-1 h-3.5 w-3.5" />{t('firmwareUpdate.installDriver')}
                  </Button>
                </div>
              )}
              {driverOK && (
                <div className="mt-2">
                  <Button size="sm" variant="ghost" className="text-muted-foreground" onClick={uninstallDriver}>
                    <Trash2 className="mr-1 h-3.5 w-3.5" />{t('firmwareUpdate.uninstallDriver')}
                  </Button>
                </div>
              )}
            </div>

            {/* 固件来源 */}
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Button size="sm" variant="outline" onClick={checkUpdates} disabled={checking}>
                  {checking ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="mr-1 h-3.5 w-3.5" />}
                  {t('firmwareUpdate.checkOfficial')}
                </Button>
                <Button size="sm" variant="outline" onClick={pickLocal}>
                  <FileUp className="mr-1 h-3.5 w-3.5" />{t('firmwareUpdate.pickLocal')}
                </Button>
              </div>

              {list.length > 0 && (
                <div className="space-y-1">
                  {list.map((fw, i) => (
                    <button
                      key={i}
                      onClick={() => { setSelected(fw); setLocalPath(''); }}
                      className={`w-full rounded-md border p-2 text-left text-sm transition ${selected === fw ? 'border-primary bg-primary/5' : 'border-border/60 hover:bg-muted/40'}`}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-medium">{fw.name || fw.version}</span>
                        <span className="text-xs text-muted-foreground">{fw.source === 'official' ? t('firmwareUpdate.official') : t('firmwareUpdate.custom')}</span>
                      </div>
                      {fw.info && <div className="mt-0.5 text-xs text-muted-foreground">{fw.info}</div>}
                    </button>
                  ))}
                </div>
              )}

              {selected?.source === 'local' && (
                <div className="truncate rounded-md border border-primary bg-primary/5 p-2 text-xs">{localPath}</div>
              )}
            </div>

            {/* 刷写 */}
            <div className="space-y-2">
              <Button
                className="w-full"
                disabled={!selected || !inISP || flashing}
                onClick={startFlash}
              >
                {flashing ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Download className="mr-1 h-4 w-4" />}
                {flashing ? t('firmwareUpdate.flashing') : t('firmwareUpdate.flashNow')}
              </Button>

              {progress && (
                <div>
                  <div className="mb-1 flex justify-between text-xs text-muted-foreground">
                    <span>{progress.phase === 'verify' ? t('firmwareUpdate.verifying') : t('firmwareUpdate.writing')}</span>
                    <span>{progress.pct}%</span>
                  </div>
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                    <div className="h-full bg-primary transition-all" style={{ width: `${progress.pct}%` }} />
                  </div>
                </div>
              )}

              {logLines.length > 0 && (
                <div ref={logRef} className="max-h-32 overflow-auto rounded-md bg-muted/40 p-2 font-mono text-[11px] leading-relaxed text-muted-foreground">
                  {logLines.map((l, i) => <div key={i}>{l}</div>)}
                </div>
              )}
            </div>

            <p className="text-[11px] text-muted-foreground">{t('firmwareUpdate.footer')}</p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Badge({ ok, children }: { ok: boolean; children: React.ReactNode }) {
  return (
    <span className={`rounded-full px-2 py-0.5 ${ok ? 'bg-emerald-500/15 text-emerald-500' : 'bg-muted text-muted-foreground'}`}>
      {children}
    </span>
  );
}
