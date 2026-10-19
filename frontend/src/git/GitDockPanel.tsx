import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { shortCommit } from '../lib/gitHelpers';
import { useDock } from '../layout/DockContext';
import type { GitDiffFile } from '../api/types';
import { GitDiffInline } from './GitDiffInline';
import { GitHistoryList } from './GitHistoryList';

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-1 pb-1 text-[10px] font-semibold uppercase tracking-wider text-tx-3 select-none">
      {children}
    </div>
  );
}

/** Секция «Изменения»: строки changed-файлов с инлайн-диффом;
 *  целиком скрывается, когда изменённых файлов нет. */
function ChangesSection({ courseSlug, onOpenInEditor }: {
  courseSlug: string;
  onOpenInEditor: (path: string) => void;
}) {
  const { data } = useQuery({
    queryKey: ['gitDiff', courseSlug],
    queryFn: () => api.gitDiff(courseSlug),
    retry: false,
  });
  const files = data?.files ?? [];
  if (files.length === 0) return null;

  return (
    <div>
      <SectionLabel>Изменения</SectionLabel>
      <div className="space-y-1">
        {files.map((f: GitDiffFile) => (
          <GitDiffInline key={f.path} file={f} onOpenInEditor={onOpenInEditor} />
        ))}
      </div>
    </div>
  );
}

function BranchesSection({ courseSlug }: { courseSlug: string }) {
  const qc = useQueryClient();
  const [target, setTarget] = useState('');
  const [confirmForce, setConfirmForce] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const { data: status } = useQuery({
    queryKey: ['gitStatus', courseSlug],
    queryFn: () => api.gitStatus(courseSlug),
    retry: false,
  });
  const { data: branches } = useQuery({
    queryKey: ['gitBranches', courseSlug],
    queryFn: () => api.gitBranches(courseSlug),
    enabled: !!status,
    retry: false,
  });

  const checkoutMut = useMutation({
    mutationFn: (force: boolean) => api.gitCheckout(courseSlug, target, force),
    onSuccess: () => {
      setErr(null);
      setConfirmForce(false);
      setTarget('');
      qc.invalidateQueries({ queryKey: ['course', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitStatus', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitBranches', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitDiff', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitLog', courseSlug] });
    },
    onError: (e) => setErr(e instanceof Error ? e.message : String(e)),
  });

  if (!status || !branches) return null;

  const current = branches.current || status.branch;
  const list = branches.branches.length > 0 ? branches.branches : [current];
  const busy = checkoutMut.isPending;

  const onSwitch = () => {
    if (!target || target === current) return;
    if (status.dirty) setConfirmForce(true);
    else checkoutMut.mutate(false);
  };

  return (
    <div>
      <SectionLabel>Ветки</SectionLabel>
      <div className="space-y-0.5" role="radiogroup">
        {list.map((b) => {
          const isCurrent = b === current;
          const isSelected = target === b || (!target && isCurrent);
          return (
            <button
              key={b}
              type="button"
              role="radio"
              aria-checked={isSelected}
              onClick={() => setTarget(b)}
              className={clsx(
                'w-full flex items-center gap-2 px-1.5 py-1 rounded text-left transition-colors cursor-pointer min-w-0',
                isSelected && !isCurrent
                  ? 'bg-bg-4 text-tx-1'
                  : isCurrent
                    ? 'text-tx-1 font-medium'
                    : 'text-tx-2 hover:bg-bg-4/60',
              )}
              title={isCurrent ? 'Текущая ветка' : `Переключиться на ${b}`}
            >
              <span
                className={clsx(
                  'w-2.5 h-2.5 rounded-full border-2 shrink-0 flex items-center justify-center',
                  isCurrent
                    ? 'border-brand'
                    : isSelected
                      ? 'border-tx-3'
                      : 'border-bdr-s',
                )}
              >
                {isCurrent && <span className="w-1 h-1 rounded-full bg-brand" />}
              </span>
              <span className="truncate font-mono text-[11px]" dir="ltr">{b}</span>
            </button>
          );
        })}
      </div>
      <button
        type="button"
        onClick={onSwitch}
        disabled={busy || !target || target === current}
        className={clsx(
          'mt-1.5 w-full px-2 py-1 rounded-md text-[11px] font-medium transition-colors',
          target && target !== current
            ? 'bg-brand text-white hover:bg-brand-hover cursor-pointer'
            : 'bg-bg-3 text-tx-3 cursor-not-allowed',
        )}
      >
        {busy ? 'Переключение...' : target && target !== current ? `Переключить на ${target}` : 'Выберите ветку'}
      </button>

      <ConfirmDialog
        open={confirmForce}
        title="Переключиться принудительно?"
        message="Есть несохранённые изменения в файлах курса — они будут потеряны при переключении ветки."
        confirmLabel={busy ? 'Переключение...' : 'Переключиться'}
        onConfirm={() => checkoutMut.mutate(true)}
        onCancel={() => setConfirmForce(false)}
      />
      {err && <div className="mt-1 px-1 text-[11px] text-err break-words">{err}</div>}
    </div>
  );
}

// PullModeDialog: three-way choice when the working tree is dirty — merge
// (stash + ff + reapply), discard (hard reset), or cancel. Deliberately its
// own dialog: ConfirmDialog maps cancel→an action, which would be dangerous
// here (a backdrop click would trigger the destructive option).
function PullModeDialog({ open, pending, onClose, onMode }: {
  open: boolean;
  pending: boolean;
  onClose: () => void;
  onMode: (mode: 'merge' | 'force') => void;
}) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-bg-3 border border-bdr rounded-xl p-6 w-full max-w-sm mx-4 shadow-xl space-y-4"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-tx-1 font-semibold text-base">Обновить курс?</h3>
        <p className="text-tx-2 text-sm">
          Есть локальные изменения файлов курса. Обновление можно подтянуть двумя способами:
        </p>
        <div className="space-y-2">
          <button
            type="button"
            disabled={pending}
            onClick={() => onMode('merge')}
            className="w-full text-left px-4 py-3 rounded-lg bg-brand/10 border border-brand/30 hover:bg-brand/20 transition-colors cursor-pointer disabled:opacity-40"
          >
            <div className="text-sm font-medium text-tx-1">Объединить</div>
            <div className="text-[11px] text-tx-3 mt-0.5">
              Изменения будут спрятаны, обновление применено, изменения вернутся поверх.
              При конфликте они сохранятся в git-stash — ничего не потеряется.
            </div>
          </button>
          <button
            type="button"
            disabled={pending}
            onClick={() => onMode('force')}
            className="w-full text-left px-4 py-3 rounded-lg bg-err/10 border border-err/30 hover:bg-err/20 transition-colors cursor-pointer disabled:opacity-40"
          >
            <div className="text-sm font-medium text-err">Затереть и обновить</div>
            <div className="text-[11px] text-tx-3 mt-0.5">
              Локальные изменения будут отброшены полностью, курс станет точно как в репозитории.
            </div>
          </button>
        </div>
        <div className="flex justify-end">
          <button
            type="button"
            onClick={onClose}
            disabled={pending}
            className="px-4 py-2 rounded-lg text-sm text-tx-2 hover:text-tx-1 transition-colors cursor-pointer"
          >
            Отмена
          </button>
        </div>
      </div>
    </div>
  );
}

function RefreshSection({ courseSlug }: { courseSlug: string }) {
  const qc = useQueryClient();
  const [confirmPullMode, setConfirmPullMode] = useState<'merge' | 'discard' | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const { data: status } = useQuery({
    queryKey: ['gitStatus', courseSlug],
    queryFn: () => api.gitStatus(courseSlug),
    retry: false,
  });

  const pullMut = useMutation({
    mutationFn: (mode: 'merge' | 'force') => api.gitPull(courseSlug, mode),
    onSuccess: () => {
      setErr(null);
      qc.invalidateQueries({ queryKey: ['course', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitStatus', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitBranches', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitDiff', courseSlug] });
      qc.invalidateQueries({ queryKey: ['gitLog', courseSlug] });
      qc.invalidateQueries({ queryKey: ['progress', courseSlug] });
    },
    onError: (e) => setErr(e instanceof Error ? e.message : String(e)),
  });

  if (!status) return null;

  const onPull = () => {
    if (status.dirty) setConfirmPullMode('merge');
    else pullMut.mutate('merge');
  };

  return (
    <div>
      <button
        type="button"
        onClick={onPull}
        disabled={pullMut.isPending}
        title="Загрузить последние изменения из репозитория"
        className="w-full flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-md border border-bdr bg-bg-3 text-xs font-medium text-tx-2 hover:text-tx-1 hover:bg-bg-4 disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
      >
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={pullMut.isPending ? 'animate-spin' : ''}>
          <path d="M2.5 2v4h4" />
          <path d="M3.5 10a6 6 0 1 0 1.5-6.5L2.5 6" />
        </svg>
        {pullMut.isPending ? 'Обновляю...' : 'Обновить курс'}
      </button>
      {err && <div className="mt-1 px-1 text-[11px] text-err break-words">{err}</div>}

      <PullModeDialog
        open={confirmPullMode !== null}
        pending={pullMut.isPending}
        onClose={() => setConfirmPullMode(null)}
        onMode={(m) => { setConfirmPullMode(null); pullMut.mutate(m); }}
      />
    </div>
  );
}

/** Git-док: status-шапка, обновление, изменения+дифф, ветки, история.
 *  Рендерится только для git-курсов (404 status endpoint = не git-курс). */
export function GitDockPanel({ courseSlug }: { courseSlug: string }) {
  const { togglePanel, openDiffFile } = useDock();

  const { data: status } = useQuery({
    queryKey: ['gitStatus', courseSlug],
    queryFn: () => api.gitStatus(courseSlug),
    retry: false,
  });

  const { data: branches } = useQuery({
    queryKey: ['gitBranches', courseSlug],
    queryFn: () => api.gitBranches(courseSlug),
    enabled: !!status,
    retry: false,
  });

  if (!status) return null;
  const current = branches?.current || status.branch;

  return (
    <div className="w-full h-full flex flex-col text-xs">
      {/* Шапка: ветка, короткий коммит, dirty-точка */}
      <div className="h-11 shrink-0 px-3 border-b border-bdr flex items-center justify-between gap-2 select-none">
        <div className="flex items-center gap-1.5 min-w-0" title={`Ветка ${current}, коммит ${shortCommit(status.commit)}`}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-tx-3">
            <line x1="6" y1="3" x2="6" y2="15" />
            <circle cx="18" cy="6" r="3" />
            <circle cx="6" cy="18" r="3" />
            <path d="M18 9a9 9 0 0 1-9 9" />
          </svg>
          <span className="truncate font-mono text-[11px] text-tx-1" dir="ltr">
            {current}
            {status.commit ? <span className="text-tx-3"> @ {shortCommit(status.commit)}</span> : null}
          </span>
          {status.dirty && (
            <span
              className="w-1.5 h-1.5 rounded-full bg-warn shrink-0"
              title="Есть несохранённые локальные изменения"
            />
          )}
        </div>
        <button
          type="button"
          onClick={() => togglePanel('git')}
          title="Скрыть git"
          className="w-6 h-6 flex items-center justify-center rounded-md text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer shrink-0"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-2 py-2 space-y-3 min-w-0">
        <RefreshSection courseSlug={courseSlug} />
        <ChangesSection courseSlug={courseSlug} onOpenInEditor={openDiffFile} />
        <BranchesSection courseSlug={courseSlug} />
        <div>
          <SectionLabel>История</SectionLabel>
          <GitHistoryList courseSlug={courseSlug} />
        </div>
      </div>
    </div>
  );
}
