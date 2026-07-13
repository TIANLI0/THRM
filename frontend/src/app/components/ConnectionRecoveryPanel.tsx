'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, FileArchive, RefreshCw, Stethoscope } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { apiService } from '../services/api';

interface ConnectionRecoveryPanelProps {
  connected: boolean;
  coreError?: string | null;
  temperatureUnavailable?: boolean;
  onRetry: () => Promise<void> | void;
}

export default function ConnectionRecoveryPanel({
  connected,
  coreError,
  temperatureUnavailable,
  onRetry,
}: ConnectionRecoveryPanelProps) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState<'retry' | 'diagnostics' | 'temperature' | null>(null);
  const needsHelp = !connected || !!coreError || !!temperatureUnavailable;
  if (!needsHelp) return null;

  const retry = async () => {
    setBusy('retry');
    try {
      await onRetry();
      toast.success(t('connectionRecovery.toasts.retryStarted'));
    } catch (error) {
      toast.error(t('connectionRecovery.toasts.recoveryFailed'), { description: error instanceof Error ? error.message : String(error) });
    } finally {
      setBusy(null);
    }
  };

  const testTemperature = async () => {
    setBusy('temperature');
    try {
      const result = await apiService.testTemperatureReading();
      if ((result?.controlTemp ?? 0) > 0) {
        toast.success(t('connectionRecovery.toasts.temperatureRecovered', { temperature: result.controlTemp }));
      } else {
        toast.warning(t('connectionRecovery.toasts.temperatureUnavailable'), { description: t('connectionRecovery.toasts.temperatureUnavailableDescription') });
      }
    } catch (error) {
      toast.error(t('connectionRecovery.toasts.temperatureTestFailed'), { description: error instanceof Error ? error.message : String(error) });
    } finally {
      setBusy(null);
    }
  };

  const exportDiagnostics = async () => {
    setBusy('diagnostics');
    try {
      const path = await apiService.exportDiagnosticPackage();
      if (path) toast.success(t('connectionRecovery.toasts.diagnosticsExported'), { description: path });
    } catch (error) {
      toast.error(t('connectionRecovery.toasts.diagnosticsExportFailed'), { description: error instanceof Error ? error.message : String(error) });
    } finally {
      setBusy(null);
    }
  };

  const steps = [
    t('connectionRecovery.steps.connection'),
    t('connectionRecovery.steps.release'),
    t('connectionRecovery.steps.diagnostics'),
  ];

  return (
    <section
      aria-live="polite"
      className="mx-auto flex w-full max-w-2xl flex-col items-center rounded-xl border border-border bg-card px-6 py-7 text-center shadow-sm shadow-black/5 min-[1800px]:max-w-3xl min-[1800px]:px-8 min-[1800px]:py-8"
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-muted text-muted-foreground">
        <AlertTriangle className="h-5 w-5" />
      </div>

      <h2 className="mt-3 text-base font-semibold text-foreground">
        {!connected ? t('connectionRecovery.title') : t('connectionRecovery.temperatureTitle')}
      </h2>
      {coreError && (
        <p className="mt-1.5 max-w-md break-words text-xs leading-relaxed text-destructive">{coreError}</p>
      )}

      <ol className="mt-5 grid w-full gap-2.5 text-left sm:grid-cols-3">
        {steps.map((text, index) => (
          <li
            key={index}
            className="flex flex-col gap-2 rounded-xl border border-border/70 bg-muted/30 p-3.5"
          >
            <span className="flex h-5 w-5 items-center justify-center rounded-full bg-primary/10 text-[11px] font-semibold text-primary">
              {index + 1}
            </span>
            <span className="text-xs leading-relaxed text-muted-foreground">{text}</span>
          </li>
        ))}
      </ol>

      <div className="mt-5 flex flex-wrap items-center justify-center gap-2.5">
        {!connected && (
          <Button size="sm" onClick={retry} disabled={busy !== null}>
            <RefreshCw className={busy === 'retry' ? 'animate-spin' : ''} />
            {t('connectionRecovery.actions.retry')}
          </Button>
        )}
        {temperatureUnavailable && (
          <Button size="sm" variant="outline" onClick={testTemperature} disabled={busy !== null}>
            <Stethoscope className={busy === 'temperature' ? 'animate-pulse' : ''} />
            {t('connectionRecovery.actions.temperatureTest')}
          </Button>
        )}
        <Button size="sm" variant="outline" onClick={exportDiagnostics} disabled={busy !== null}>
          <FileArchive className={busy === 'diagnostics' ? 'animate-pulse' : ''} />
          {t('connectionRecovery.actions.exportDiagnostics')}
        </Button>
      </div>
    </section>
  );
}
