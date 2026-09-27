import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { ConfirmDialog } from './ui/ConfirmDialog';
import { shortCommit } from '../lib/gitHelpers';

// Compact row shown on a course page when the course was imported from a git
// repo. Lets the user switch branches or pull the latest commits. Renders nothing
// when the course has no git source (404 from the status endpoint).
export function GitSourcePanel({ courseSlug }: { courseSlug: string }) {
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
    <div className="shrink-0 px-3 py-2 border-b border-bdr bg-bg-2/60 flex flex-wrap items-center gap-2 text-xs">
      <span className="flex items-center gap-1.5 text-tx-3 font-medium" title={branches?.source?.repo}>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <line x1="6" y1="3" x2="6" y2="15" />
          <circle cx="18" cy="6" r="3" />
          <circle cx="6" cy="18" r="3" />
          <path d="M18 9a9 9 0 0 1-9 9" />
        </svg>
        <span>git</span>
      </span>

      <select
        value={target || current}
        onChange={(e) => setTarget(e.target.value)}
        disabled={busy}
        className="px-2 py-1 rounded-md bg-bg-3 border border-bdr text-tx-1 text-xs focus:border-brand focus:outline-none font-mono cursor-pointer disabled:opacity-50"
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
        className="px-2.5 py-1 rounded-md bg-brand text-white text-xs font-medium hover:bg-brand-hover disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
      >
        {checkoutMut.isPending ? '...' : 'Переключить'}
      </button>

      <button
        type="button"
        onClick={onPull}
        disabled={busy}
        title="Загрузить последние изменения"
        className="px-2.5 py-1 rounded-md border border-bdr bg-bg-3 text-tx-2 hover:text-tx-1 hover:bg-bg-4 text-xs font-medium disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
      >
        {pullMut.isPending ? '...' : 'Обновить'}
      </button>

      <span className="text-tx-3 font-mono ml-auto" title={`Коммит ${status.commit}`}>
        {current}{status.commit ? ` @ ${shortCommit(status.commit)}` : ''}
        {status.dirty && <span className="text-warn ml-1.5" title="Есть несохранённые локальные изменения">●</span>}
      </span>

      {err && <span className="basis-full text-err text-[11px]">{err}</span>}

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
