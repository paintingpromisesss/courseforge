import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { Markdown } from './ui/Markdown';

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let val = bytes;
  let unitIndex = 0;
  while (val >= 1024 && unitIndex < units.length - 1) {
    val /= 1024;
    unitIndex++;
  }
  return `${val.toFixed(1)} ${units[unitIndex]}`;
}

function formatRelativeTime(dateStr?: string): string {
  if (!dateStr) return 'не проверялось';
  const date = new Date(dateStr);
  const now = new Date();
  const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diffSec < 60) return 'только что';
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} мин. назад`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} ч. назад`;
  return date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
}

function CheckIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function RefreshIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.19" />
    </svg>
  );
}

function DownloadIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="7 10 12 15 17 10" />
      <line x1="12" y1="15" x2="12" y2="3" />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
      <polyline points="15 3 21 3 21 9" />
      <line x1="10" y1="14" x2="21" y2="3" />
    </svg>
  );
}

function CopyIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function AnvilIcon() {
  return (
    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 10H6a4 4 0 0 1-4-4 1 1 0 0 1 1-1h18a1 1 0 0 1 1 1 4 4 0 0 1-4 4h-1" />
      <path d="M9 10v4a3 3 0 0 0 3 3h0a3 3 0 0 0 3-3v-4" />
      <path d="M5 20h14" />
      <path d="M9 17v3" />
      <path d="M15 17v3" />
    </svg>
  );
}

export function AboutSection() {
  const qc = useQueryClient();
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [isRestarting, setIsRestarting] = useState(false);

  // Poll version status frequently while downloading, otherwise standard 30s
  const { data: verInfo } = useQuery({
    queryKey: ['version'],
    queryFn: api.getVersion,
    refetchInterval: (query) => {
      const state = query.state.data?.status?.state;
      return state === 'downloading' ? 800 : 30_000;
    },
  });

  const checkMutation = useMutation({
    mutationFn: api.checkUpdate,
    onSuccess: (data) => {
      qc.setQueryData(['version'], (prev: typeof verInfo) => {
        if (!prev) return prev;
        return {
          ...prev,
          check: data,
        };
      });
      qc.invalidateQueries({ queryKey: ['version'] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: api.startUpdate,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['version'] });
    },
  });

  const restartMutation = useMutation({
    mutationFn: api.restartServer,
    onSuccess: () => {
      setIsRestarting(true);
    },
  });

  // When restarting, poll until server answers and then reload
  useEffect(() => {
    if (!isRestarting) return;
    const interval = setInterval(async () => {
      try {
        await api.getVersion();
        window.location.reload();
      } catch {
        // Still restarting
      }
    }, 1200);
    return () => clearInterval(interval);
  }, [isRestarting]);

  const copyToClipboard = async (text: string, key: string) => {
    await navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 1500);
  };

  const status = verInfo?.status;
  const check = verInfo?.check;
  const currentVersion = verInfo?.version ?? 'dev';
  const hasUpdate = Boolean(check?.update_available);
  const release = check?.release;

  return (
    <div className="space-y-6 max-w-3xl">
      {/* ── Brand Hero Header ────────────────────────────────────────────── */}
      <div className="relative overflow-hidden rounded-2xl border border-bdr bg-gradient-to-br from-bg-2/80 via-bg-1 to-bg-2/40 p-6 shadow-sm">
        <div className="absolute -right-6 -bottom-6 w-36 h-36 rounded-full bg-brand/10 blur-2xl pointer-events-none" />

        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 relative z-10">
          <div className="flex items-center gap-4">
            <div className="w-14 h-14 rounded-2xl bg-brand/10 border border-brand/25 flex items-center justify-center text-brand shadow-inner shrink-0">
              <AnvilIcon />
            </div>
            <div>
              <div className="flex items-center gap-2.5 flex-wrap">
                <h2 className="text-xl font-bold tracking-tight text-tx-1">CourseForge</h2>
                <span className="font-mono text-xs px-2 py-0.5 rounded-full bg-brand/15 text-brand border border-brand/30 font-medium">
                  {currentVersion}
                </span>
                {verInfo?.is_dev && (
                  <span className="text-[10px] uppercase tracking-wider font-semibold px-1.5 py-0.5 rounded bg-warn/15 text-warn border border-warn/30">
                    dev build
                  </span>
                )}
              </div>
              <p className="text-xs text-tx-3 mt-1 leading-relaxed max-w-lg">
                Интерактивная среда для решения алгоритмических задач и изучения языков программирования с автоматической проверкой решений
              </p>
            </div>
          </div>

          <div className="flex flex-wrap sm:flex-col items-end gap-1.5 text-[11px] font-mono text-tx-3 shrink-0">
            <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-bg-3 border border-bdr">
              <span className="text-tx-2 font-medium">{verInfo?.os ?? 'os'}</span>
              <span>/</span>
              <span className="text-tx-2">{verInfo?.arch ?? 'arch'}</span>
            </div>
            <div className="text-[10px] text-tx-3/80 px-1">
              Go runtime: {verInfo?.go_version ?? 'go'}
            </div>
          </div>
        </div>
      </div>

      {/* ── Update Center Card ───────────────────────────────────────────── */}
      <div className="rounded-2xl border border-bdr bg-bg-2/60 p-5 space-y-4">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h3 className="text-sm font-semibold text-tx-1">Центр обновлений</h3>
            <p className="text-xs text-tx-3 mt-0.5">
              Проверка релизов на GitHub и обновление исполняемого файла
            </p>
          </div>

          <button
            type="button"
            onClick={() => checkMutation.mutate()}
            disabled={checkMutation.isPending || status?.state === 'downloading' || isRestarting}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 hover:text-tx-1 text-tx-2 text-xs font-medium transition-all disabled:opacity-50 cursor-pointer"
            title="Проверить наличие новых релизов"
          >
            <span className={clsx('shrink-0', checkMutation.isPending && 'animate-spin')}>
              <RefreshIcon />
            </span>
            <span>{checkMutation.isPending ? 'Проверка…' : 'Проверить'}</span>
          </button>
        </div>

        {/* Status display states */}
        {isRestarting ? (
          <div className="rounded-xl border border-brand/40 bg-brand/10 p-4 text-center space-y-2">
            <div className="inline-block animate-spin text-brand">
              <RefreshIcon />
            </div>
            <div className="text-xs font-semibold text-tx-1">CourseForge перезапускается…</div>
            <p className="text-[11px] text-tx-3">Страница автоматически обновится после перезапуска сервера.</p>
          </div>
        ) : status?.state === 'downloading' ? (
          <div className="rounded-xl border border-brand/30 bg-bg-3 p-4 space-y-3">
            <div className="flex items-center justify-between text-xs">
              <span className="font-medium text-tx-1">
                {status.message || 'Скачивание обновления…'}
              </span>
              <span className="font-mono text-brand font-semibold">{status.progress}%</span>
            </div>
            {/* Progress bar */}
            <div className="w-full h-2 rounded-full bg-bg-4 overflow-hidden border border-bdr">
              <div
                className="h-full bg-brand transition-all duration-300 rounded-full"
                style={{ width: `${Math.max(5, status.progress)}%` }}
              />
            </div>
            <p className="text-[11px] text-tx-3">Пожалуйста, не закрывайте CourseForge во время обновления.</p>
          </div>
        ) : status?.state === 'ready_restart' ? (
          <div className="rounded-xl border border-ok/40 bg-ok/10 p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div>
              <div className="flex items-center gap-1.5 text-xs font-semibold text-ok">
                <CheckIcon />
                <span>Обновление успешно установлено на диск</span>
              </div>
              <p className="text-[11px] text-tx-2 mt-0.5">
                Бинарный файл заменен на версию {check?.latest_version || 'новую'}. Нажмите кнопку для перезапуска.
              </p>
            </div>
            <button
              type="button"
              onClick={() => restartMutation.mutate()}
              disabled={restartMutation.isPending}
              className="px-4 py-2 rounded-xl bg-ok hover:bg-ok/90 text-white text-xs font-semibold shadow-sm transition-opacity cursor-pointer shrink-0"
            >
              {restartMutation.isPending ? 'Запуск…' : 'Перезапустить сейчас'}
            </button>
          </div>
        ) : status?.state === 'error' ? (
          <div className="rounded-xl border border-err/40 bg-err/10 p-4 flex items-center justify-between gap-3">
            <div>
              <div className="text-xs font-semibold text-err">Ошибка обновления</div>
              <p className="text-[11px] text-tx-2 mt-0.5">{status.error || 'Не удалось обновить бинарник'}</p>
            </div>
            <button
              type="button"
              onClick={() => updateMutation.mutate()}
              className="px-3 py-1.5 rounded-lg border border-err/30 bg-err/20 hover:bg-err/30 text-err text-xs font-medium transition-colors"
            >
              Повторить
            </button>
          </div>
        ) : hasUpdate ? (
          <div className="rounded-xl border border-brand/35 bg-brand/5 p-4.5 space-y-3">
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2">
                  <span className="flex h-2 w-2 rounded-full bg-brand animate-pulse" />
                  <span className="text-xs font-semibold text-tx-1">
                    Доступна новая версия {check?.latest_version}
                  </span>
                  {release?.asset_size ? (
                    <span className="text-[11px] text-tx-3 font-mono">
                      ({formatBytes(release.asset_size)})
                    </span>
                  ) : null}
                </div>
                <div className="text-[11px] text-tx-3 mt-1">
                  Текущая версия: <span className="font-mono text-tx-2">{currentVersion}</span> •{' '}
                  Опубликовано: {new Date(release?.published_at || '').toLocaleDateString('ru-RU')}
                </div>
              </div>

              <div className="flex items-center gap-2 shrink-0">
                {release?.html_url && (
                  <a
                    href={release.html_url}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-1 px-3 py-1.5 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 text-tx-2 text-xs font-medium transition-colors"
                  >
                    <span>Релиз</span>
                    <ExternalLinkIcon />
                  </a>
                )}
                <button
                  type="button"
                  onClick={() => updateMutation.mutate()}
                  disabled={updateMutation.isPending}
                  className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-brand hover:bg-brand-hover text-white text-xs font-semibold shadow-sm transition-all cursor-pointer"
                >
                  <DownloadIcon />
                  <span>{updateMutation.isPending ? 'Загрузка…' : `Обновить до ${check?.latest_version}`}</span>
                </button>
              </div>
            </div>

            {/* Changelog preview */}
            {release?.body && (
              <div className="mt-3 pt-3 border-t border-bdr/60">
                <div className="text-[11px] font-semibold text-tx-2 mb-1.5">Что нового:</div>
                <div className="max-h-48 overflow-y-auto rounded-xl bg-bg-1/70 border border-bdr p-3.5 text-xs text-tx-2 prose prose-invert prose-xs max-w-none">
                  <Markdown content={release.body} />
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="rounded-xl border border-bdr bg-bg-3/40 p-4 flex items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-full bg-ok/10 border border-ok/30 flex items-center justify-center text-ok shrink-0">
                <CheckIcon />
              </div>
              <div>
                <div className="text-xs font-semibold text-tx-1">У вас установлена актуальная версия</div>
                <div className="text-[11px] text-tx-3 mt-0.5">
                  Проверено: {formatRelativeTime(check?.checked_at)}
                </div>
              </div>
            </div>
            {release?.html_url && (
              <a
                href={release.html_url}
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-1 text-[11px] text-tx-3 hover:text-brand transition-colors"
              >
                <span>История версий</span>
                <ExternalLinkIcon />
              </a>
            )}
          </div>
        )}
      </div>

      {/* ── System Details & Directories ─────────────────────────────────── */}
      <div className="rounded-2xl border border-bdr bg-bg-2/40 p-5 space-y-3">
        <h3 className="text-xs font-semibold text-tx-2 uppercase tracking-wider">
          Рабочее окружение
        </h3>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="rounded-xl border border-bdr bg-bg-3/60 p-3 flex items-center justify-between gap-2">
            <div className="min-w-0">
              <div className="text-[11px] text-tx-3">Каталог курсов</div>
              <div className="text-xs font-mono text-tx-1 truncate" title="courses">
                ./courses
              </div>
            </div>
            <button
              type="button"
              onClick={() => copyToClipboard('courses', 'courses')}
              className="p-1.5 rounded-lg text-tx-3 hover:text-tx-1 hover:bg-bg-4 transition-colors shrink-0"
              title="Копировать путь"
            >
              {copiedKey === 'courses' ? <CheckIcon /> : <CopyIcon />}
            </button>
          </div>

          <div className="rounded-xl border border-bdr bg-bg-3/60 p-3 flex items-center justify-between gap-2">
            <div className="min-w-0">
              <div className="text-[11px] text-tx-3">Каталог данных и состояния</div>
              <div className="text-xs font-mono text-tx-1 truncate" title="data">
                ./data
              </div>
            </div>
            <button
              type="button"
              onClick={() => copyToClipboard('data', 'data')}
              className="p-1.5 rounded-lg text-tx-3 hover:text-tx-1 hover:bg-bg-4 transition-colors shrink-0"
              title="Копировать путь"
            >
              {copiedKey === 'data' ? <CheckIcon /> : <CopyIcon />}
            </button>
          </div>
        </div>

        {/* Project Links */}
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2 pt-2 border-t border-bdr/60 text-xs text-tx-3">
          <a
            href="https://github.com/paintingpromisesss/courseforge"
            target="_blank"
            rel="noreferrer"
            className="hover:text-brand transition-colors flex items-center gap-1"
          >
            <span>GitHub репозиторий</span>
            <ExternalLinkIcon />
          </a>
          <a
            href="https://github.com/paintingpromisesss/courseforge/releases"
            target="_blank"
            rel="noreferrer"
            className="hover:text-brand transition-colors flex items-center gap-1"
          >
            <span>Все релизы</span>
            <ExternalLinkIcon />
          </a>
          <a
            href="https://github.com/paintingpromisesss/courseforge/issues"
            target="_blank"
            rel="noreferrer"
            className="hover:text-brand transition-colors flex items-center gap-1"
          >
            <span>Сообщить об ошибке</span>
            <ExternalLinkIcon />
          </a>
          <span className="ml-auto text-[11px] text-tx-3/70">
            MIT License • CourseForge
          </span>
        </div>
      </div>
    </div>
  );
}
