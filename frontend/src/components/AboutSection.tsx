import { useState, useEffect, lazy, Suspense } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { api } from '../api/client';

const Markdown = lazy(() => import('./ui/Markdown').then((m) => ({ default: m.Markdown })));

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

function cleanChangelog(raw: string): string {
  if (!raw) return '';
  // 1. Remove duplicate leading headers (e.g. "## What's Changed" or "## Что нового")
  let text = raw.replace(/^#+\s*(?:What's Changed|Что нового)[^\n]*\n+/i, '').trim();

  // 2. Shorten raw GitHub PR URLs like https://github.com/paintingpromisesss/courseforge/pull/15 to #15
  text = text.replace(/https:\/\/github\.com\/[^\s/]+\/[^\s/]+\/pull\/(\d+)/g, '#$1');

  // 3. Remove "Full Changelog: https://..." footer line
  text = text.replace(/\*\*Full Changelog\*\*:[^\n]*/gi, '').trim();

  return text;
}

function splitChangelog(body?: string): { ru: string; en: string; isBilingual: boolean } {
  if (!body) return { ru: '', en: '', isBilingual: false };

  const ruMatch = body.search(/##\s+Что нового/i);
  const enMatch = body.search(/##\s+What's Changed/i);

  if (ruMatch !== -1 && enMatch !== -1) {
    if (ruMatch > enMatch) {
      const enPart = body.slice(enMatch, ruMatch).replace(/---\s*$/, '').trim();
      const ruPart = body.slice(ruMatch).trim();
      return { ru: cleanChangelog(ruPart), en: cleanChangelog(enPart), isBilingual: true };
    } else {
      const ruPart = body.slice(ruMatch, enMatch).replace(/---\s*$/, '').trim();
      const enPart = body.slice(enMatch).trim();
      return { ru: cleanChangelog(ruPart), en: cleanChangelog(enPart), isBilingual: true };
    }
  }

  const cleaned = cleanChangelog(body);
  return { ru: cleaned, en: cleaned, isBilingual: false };
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

function CourseForgeLogoIcon() {
  return (
    <svg
      width="28"
      height="28"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M7 10H6a4 4 0 0 1-4-4 1 1 0 0 1 1-1h4" />
      <path d="M7 5a1 1 0 0 1 1-1h13a1 1 0 0 1 1 1 7 7 0 0 1-7 7H8a1 1 0 0 1-1-1z" />
      <path d="M9 12v5" />
      <path d="M15 12v5" />
      <path d="M5 20a3 3 0 0 1 3-3h8a3 3 0 0 1 3 3 1 1 0 0 1-1 1H6a1 1 0 0 1-1-1" />
    </svg>
  );
}

export function AboutSection() {
  const qc = useQueryClient();
  const [countdown, setCountdown] = useState<number>(3);
  const [changelogLang, setChangelogLang] = useState<'ru' | 'en'>('ru');

  // Poll version status frequently while downloading, otherwise standard 30s
  const {
    data: verInfo,
    isError,
    refetch: refetchVersion,
  } = useQuery({
    queryKey: ['version'],
    queryFn: api.getVersion,
    refetchInterval: (query) => {
      const state = query.state.data?.status?.state;
      return state === 'downloading' || state === 'ready_restart' ? 600 : 30_000;
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

  const status = verInfo?.status;
  const check = verInfo?.check;
  const currentVersion = verInfo?.version ?? 'dev';
  const hasUpdate = Boolean(check?.update_available);
  const release = check?.release;

  // Countdown timer and auto-reload on successful update
  useEffect(() => {
    if (status?.state !== 'ready_restart') return;

    const timer = setInterval(() => {
      setCountdown((prev) => (prev > 1 ? prev - 1 : 0));
    }, 1000);

    // After 2.5s (when backend triggers its 3s restart), poll /api/version and reload
    const pollTimer = setTimeout(() => {
      let attempts = 0;
      const interval = setInterval(async () => {
        attempts++;
        try {
          await api.getVersion();
          clearInterval(interval);
          window.location.reload();
        } catch {
          if (attempts > 30) clearInterval(interval);
        }
      }, 600);
    }, 2500);

    return () => {
      clearInterval(timer);
      clearTimeout(pollTimer);
    };
  }, [status?.state]);

  return (
    <div className="space-y-6 max-w-3xl">
      {/* ── Brand Hero Header ────────────────────────────────────────────── */}
      <div className="relative overflow-hidden rounded-2xl border border-bdr bg-gradient-to-br from-bg-2/80 via-bg-1 to-bg-2/40 p-6 shadow-sm">
        <div className="absolute -right-6 -bottom-6 w-36 h-36 rounded-full bg-brand/10 blur-2xl pointer-events-none" />

        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 relative z-10">
          <div className="flex items-center gap-4">
            <div className="w-14 h-14 rounded-2xl bg-brand/10 border border-brand/25 flex items-center justify-center text-brand shadow-inner shrink-0">
              <CourseForgeLogoIcon />
            </div>
            <div>
              <div className="flex items-center gap-2.5 flex-wrap">
                <h2 className="text-xl font-bold tracking-tight text-tx-1">CourseForge</h2>
                <span
                  className={clsx(
                    'font-mono text-xs px-2.5 py-0.5 rounded-full font-medium border',
                    verInfo?.is_dev
                      ? 'bg-warn/10 text-warn border-warn/30'
                      : 'bg-brand/15 text-brand border border-brand/30',
                  )}
                >
                  {verInfo?.is_dev && currentVersion === 'dev' ? 'dev build' : currentVersion}
                </span>
              </div>
              <p className="text-xs text-tx-3 mt-1 leading-relaxed max-w-lg">
                Интерактивная среда для решения алгоритмических задач и изучения языков программирования с автоматической проверкой решений
              </p>
            </div>
          </div>

          {verInfo?.os && verInfo?.arch ? (
            <div className="flex flex-wrap sm:flex-col items-end gap-1.5 text-[11px] font-mono text-tx-3 shrink-0">
              <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-bg-3 border border-bdr">
                <span className="text-tx-2 font-medium">{verInfo.os}</span>
                <span>/</span>
                <span className="text-tx-2">{verInfo.arch}</span>
              </div>
              {verInfo.go_version && (
                <div className="text-[10px] text-tx-3/80 px-1">
                  Go runtime: {verInfo.go_version}
                </div>
              )}
            </div>
          ) : isError ? (
            <div className="text-[11px] text-warn px-2.5 py-1 rounded-lg bg-warn/10 border border-warn/25 shrink-0">
              Нет связи с сервером
            </div>
          ) : null}
        </div>
      </div>

      {/* ── Update Center Card ───────────────────────────────────────────── */}
      <div className="rounded-2xl border border-bdr bg-bg-2/50 p-5 space-y-4">
        {/* Header row: Status + Action buttons */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span
                className={clsx(
                  'w-2 h-2 rounded-full shrink-0',
                  isError ? 'bg-err' : hasUpdate ? 'bg-brand animate-pulse' : 'bg-ok',
                )}
              />
              <span className="text-sm font-semibold text-tx-1">
                {isError
                  ? 'Не удалось проверить статус'
                  : hasUpdate
                  ? `Доступна версия ${check?.latest_version}`
                  : 'У вас установлена последняя версия'}
              </span>
            </div>

            <p className="text-xs text-tx-3 mt-1 pl-4">
              {isError ? (
                'Сервер не отвечает на запрос версии'
              ) : hasUpdate ? (
                <>
                  <span>Опубликовано: {new Date(release?.published_at || '').toLocaleDateString('ru-RU')}</span>
                  {release?.asset_size ? (
                    <span className="font-mono"> • {formatBytes(release.asset_size)}</span>
                  ) : null}
                </>
              ) : (
                check?.checked_at
                  ? `Проверено: ${formatRelativeTime(check.checked_at)}`
                  : 'Проверка ещё не выполнялась'
              )}
            </p>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <button
              type="button"
              onClick={() => (isError ? refetchVersion() : checkMutation.mutate())}
              disabled={checkMutation.isPending || status?.state === 'downloading' || status?.state === 'ready_restart'}
              className="w-8 h-8 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 hover:text-tx-1 text-tx-3 flex items-center justify-center transition-all disabled:opacity-50 cursor-pointer shrink-0"
              title="Проверить обновления на GitHub"
              aria-label="Проверить обновления"
            >
              <span className={clsx('shrink-0', checkMutation.isPending && 'animate-spin')}>
                <RefreshIcon />
              </span>
            </button>

            {hasUpdate && release?.html_url && (
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

            {hasUpdate && (
              status?.state === 'ready_restart' ? (
                <button
                  type="button"
                  disabled
                  className="flex items-center justify-center gap-1.5 min-w-[136px] px-4 py-1.5 rounded-xl bg-ok text-white text-xs font-semibold tabular-nums shadow-sm transition-all cursor-default"
                >
                  <span className="shrink-0 animate-spin">
                    <RefreshIcon />
                  </span>
                  <span>
                    {countdown > 0 ? `Перезапуск (${countdown})…` : 'Перезапуск…'}
                  </span>
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => updateMutation.mutate()}
                  disabled={updateMutation.isPending || status?.state === 'downloading'}
                  className="flex items-center justify-center gap-1.5 min-w-[100px] px-4 py-1.5 rounded-xl bg-brand hover:bg-brand-hover text-white text-xs font-semibold shadow-sm transition-all cursor-pointer disabled:opacity-60"
                >
                  <DownloadIcon />
                  <span>{updateMutation.isPending ? 'Загрузка…' : `Обновить`}</span>
                </button>
              )
            )}
          </div>
        </div>

        {/* Animated download progress bar */}
        <AnimatePresence>
          {status?.state === 'downloading' && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
              className="overflow-hidden"
            >
              <div className="pt-1">
                <div className="rounded-xl border border-brand/30 bg-bg-3/80 p-3.5 space-y-2">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-medium text-tx-1">
                      {status.message || 'Скачивание обновления…'}
                    </span>
                    <span className="font-mono text-brand font-semibold tabular-nums">{status.progress}%</span>
                  </div>
                  <div className="w-full h-1.5 rounded-full bg-bg-4 overflow-hidden border border-bdr-s">
                    <div
                      className="h-full bg-brand rounded-full transition-[width] duration-500 ease-out"
                      style={{ width: `${Math.max(4, status.progress)}%` }}
                    />
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Error notification if update failed */}
        <AnimatePresence>
          {status?.state === 'error' && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="rounded-xl border border-err/40 bg-err/10 p-3.5 flex items-center justify-between gap-3 overflow-hidden"
            >
              <div>
                <div className="text-xs font-semibold text-err">Ошибка обновления</div>
                <p className="text-[11px] text-tx-2 mt-0.5">{status.error || 'Не удалось обновить бинарник'}</p>
              </div>
              <button
                type="button"
                onClick={() => updateMutation.mutate()}
                className="px-3 py-1.5 rounded-lg border border-err/30 bg-err/20 hover:bg-err/30 text-err text-xs font-medium transition-colors cursor-pointer"
              >
                Повторить
              </button>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Server connection error */}
        {isError && (
          <div className="rounded-xl border border-err/30 bg-err/10 p-4 flex items-center justify-between gap-3">
            <div>
              <div className="text-xs font-semibold text-err">Не удалось связаться с сервером</div>
              <p className="text-[11px] text-tx-2 mt-0.5">Эндпоинт /api/version недоступен или сервер остановлен</p>
            </div>
            <button
              type="button"
              onClick={() => refetchVersion()}
              className="px-3 py-1.5 rounded-lg border border-err/30 bg-err/20 hover:bg-err/30 text-err text-xs font-medium transition-colors cursor-pointer"
            >
              Повторить
            </button>
          </div>
        )}

        {/* Changelog section if available */}
        {hasUpdate && release?.body && (() => {
          const parsed = splitChangelog(release.body);
          const content = (changelogLang === 'en' ? parsed.en : parsed.ru) || release.body;
          if (!content.trim()) return null;

          return (
            <div className="pt-4 border-t border-bdr-s space-y-2.5">
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-semibold text-tx-2">
                  {changelogLang === 'en' ? "What's new in this release" : 'Изменения в этой версии'}
                </span>

                {parsed.isBilingual && (
                  <div className="flex items-center gap-0.5 p-0.5 rounded-lg bg-bg-3 border border-bdr text-[10px] font-medium">
                    <button
                      type="button"
                      onClick={() => setChangelogLang('ru')}
                      className={clsx(
                        'px-2 py-0.5 rounded transition-colors cursor-pointer',
                        changelogLang === 'ru'
                          ? 'bg-brand text-white font-semibold'
                          : 'text-tx-3 hover:text-tx-1',
                      )}
                    >
                      RU
                    </button>
                    <button
                      type="button"
                      onClick={() => setChangelogLang('en')}
                      className={clsx(
                        'px-2 py-0.5 rounded transition-colors cursor-pointer',
                        changelogLang === 'en'
                          ? 'bg-brand text-white font-semibold'
                          : 'text-tx-3 hover:text-tx-1',
                      )}
                    >
                      EN
                    </button>
                  </div>
                )}
              </div>

              <div className="max-h-80 overflow-y-auto pr-1 text-xs text-tx-2 leading-relaxed [&_.markdown-body]:!text-xs [&_.markdown-body]:!leading-relaxed [&_.markdown-body_ul]:!my-1.5 [&_.markdown-body_ul]:!pl-4 [&_.markdown-body_li]:!my-0.5 [&_.markdown-body_p]:!my-1">
                <Suspense fallback={<div className="text-xs text-tx-3 py-2">Загрузка описания...</div>}>
                  <Markdown content={content} />
                </Suspense>
              </div>
            </div>
          );
        })()}
      </div>

      {/* ── Project Links Footer ────────────────────────────────────────── */}
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 pt-2 border-t border-bdr-s text-xs text-tx-3 px-1">
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
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
        </div>
        <span className="text-[11px] text-tx-3/70">
          MIT License • CourseForge
        </span>
      </div>
    </div>
  );
}
