import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { ConfirmDialog } from './ui/ConfirmDialog';
import { shortCommit } from '../lib/gitHelpers';

// GitTreeToggle: the small git icon in the contents-tree header. Self-checks
// whether the course is git-managed (status endpoint 404 = not) and renders
// nothing when it isn't. dirtyHint lights the icon up when there are local
// changes, so the git state is discoverable without opening the panel.
export function GitTreeToggle({ courseSlug, active, onToggle }: {
  courseSlug: string;
  active: boolean;
  onToggle: () => void;
}) {
  const { data: status } = useQuery({
    queryKey: ['gitStatus', courseSlug],
    queryFn: () => api.gitStatus(courseSlug),
    retry: false,
  });
  if (!status) return null;
  return (
    <button
      type="button"
      onClick={onToggle}
      title={status.dirty ? 'Git: есть локальные изменения' : 'Git: ветки и обновления курса'}
      className={clsx(
        'w-7 h-7 flex items-center justify-center rounded-md transition-all active:scale-95 cursor-pointer',
        active
          ? 'bg-brand/15 text-brand'
          : status.dirty
            ? 'text-warn hover:text-tx-1 hover:bg-bg-3'
            : 'text-tx-3 hover:text-tx-1 hover:bg-bg-3',
      )}
    >
      <svg
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="shrink-0"
      >
        <line x1="6" y1="3" x2="6" y2="15" />
        <circle cx="18" cy="6" r="3" />
        <circle cx="6" cy="18" r="3" />
        <path d="M18 9a9 9 0 0 1-9 9" />
      </svg>
      {status.dirty && !active && <span className="absolute w-1.5 h-1.5 rounded-full bg-warn translate-x-3 -translate-y-3" />}
    </button>
  );
}

// GitTreeControls: on-demand branch/pull block rendered inside the course
// contents tree (above tasks) when the user toggles the git icon in the tree
// header. Props: courseSlug plus hasGit so the parent can hint availability
// (the button itself needs the query result anyway, so it stays self-fetching).
export function GitTreeControls({ courseSlug, onClose }: { courseSlug: string; onClose: () => void }) {
  const qc = useQueryClient();
  const [target, setTarget] = useState('');
  const [confirmForce, setConfirmForce] = useState(false);
  const [confirmPullMode, setConfirmPullMode] = useState<'merge' | 'discard' | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const { data: status } = useQuery({
    queryKey: ['gitStatus', courseSlug],
    queryFn: () => api.gitStatus(courseSlug),
    // a non-git course returns 404; treat it as "no panel" rather than an error
    retry: false,
  });

  const { data: branches } = useQuery({
    queryKey: ['gitBranches', courseSlug],
    queryFn: () => api.gitBranches(courseSlug),
    enabled: !!status,
    retry: false,
  });

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['course', courseSlug] });
    qc.invalidateQueries({ queryKey: ['gitStatus', courseSlug] });
    qc.invalidateQueries({ queryKey: ['gitBranches', courseSlug] });
  };

  const checkoutMut = useMutation({
    mutationFn: (force: boolean) => api.gitCheckout(courseSlug, target, force),
    onSuccess: () => { setErr(null); setConfirmForce(false); refresh(); },
    onError: (e) => setErr(e instanceof Error ? e.message : String(e)),
  });

  const pullMut = useMutation({
    mutationFn: (mode: 'merge' | 'force') => api.gitPull(courseSlug, mode),
    onSuccess: () => { setErr(null); refresh(); },
    onError: (e) => setErr(e instanceof Error ? e.message : String(e)),
  });

  if (!status) return null;

  const branchList = branches?.branches ?? [];
  const current = branches?.current || status.branch;
  const busy = checkoutMut.isPending || pullMut.isPending;

  const onSwitch = () => {
    if (!target || target === current) return;
    if (status.dirty) setConfirmForce(true);
    else checkoutMut.mutate(false);
  };

  const onPull = () => {
    if (status.dirty) setConfirmPullMode('merge');
    else pullMut.mutate('merge');
  };

  return (
    <div className="mx-2 mt-2 mb-1 rounded-lg border border-bdr bg-bg-2 p-2.5 space-y-2 text-xs shrink-0">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-1.5 text-tx-3 font-medium" title={branches?.source?.repo}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="6" y1="3" x2="6" y2="15" />
            <circle cx="18" cy="6" r="3" />
            <circle cx="6" cy="18" r="3" />
            <path d="M18 9a9 9 0 0 1-9 9" />
          </svg>
          <span>git</span>
        </span>
        <button
          type="button"
          onClick={onClose}
          title="Скрыть git"
          className="w-6 h-6 flex items-center justify-center rounded-md text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <div className="flex gap-1.5">
        <select
          value={target || current}
          onChange={(e) => setTarget(e.target.value)}
          disabled={busy}
          className="flex-1 min-w-0 px-2 py-1 rounded-md bg-bg-3 border border-bdr text-tx-1 text-xs focus:border-brand focus:outline-none font-mono cursor-pointer disabled:opacity-50"
        >
          {branchList.length === 0 && <option value={current}>{current}</option>}
          {branchList.map((b) => (
            <option key={b} value={b}>{b}</option>
          ))}
        </select>
        <button
          type="button"
          onClick={onSwitch}
          disabled={busy || !target || target === current}
          className="px-2.5 py-1 rounded-md bg-brand text-white text-xs font-medium hover:bg-brand-hover disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer shrink-0"
        >
          {checkoutMut.isPending ? '...' : 'Переключить'}
        </button>
      </div>

      <button
        type="button"
        onClick={onPull}
        disabled={busy}
        title="Загрузить последние изменения"
        className="w-full px-2.5 py-1 rounded-md border border-bdr bg-bg-3 text-tx-2 hover:text-tx-1 hover:bg-bg-4 text-xs font-medium disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
      >
        {pullMut.isPending ? '...' : 'Обновить'}
      </button>

      <div className="text-tx-3 font-mono" title={`Коммит ${status.commit}`}>
        {current}{status.commit ? ` @ ${shortCommit(status.commit)}` : ''}
        {status.dirty && <span className="text-warn ml-1.5" title="Есть несохранённые локальные изменения">●</span>}
      </div>

      {err && <div className="text-err text-[11px]">{err}</div>}

      <ConfirmDialog
        open={confirmForce}
        title="Переключиться принудительно?"
        message="Есть несохранённые изменения в файлах курса — они будут потеряны при переключении ветки."
        confirmLabel={checkoutMut.isPending ? 'Переключение...' : 'Переключиться'}
        onConfirm={() => checkoutMut.mutate(true)}
        onCancel={() => setConfirmForce(false)}
      />

      <PullModeDialog
        open={confirmPullMode !== null}
        pending={pullMut.isPending}
        onClose={() => setConfirmPullMode(null)}
        onMode={(m) => { setConfirmPullMode(null); pullMut.mutate(m); }}
      />
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
