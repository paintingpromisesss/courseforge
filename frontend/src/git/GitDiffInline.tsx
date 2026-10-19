import { useState } from 'react';
import clsx from 'clsx';
import type { GitDiffFile } from '../api/types';
import { parseHunk, hunkStats } from './diffHunks';

const STATUS_BADGE: Record<GitDiffFile['status'], { label: string; cls: string; title: string }> = {
  M: { label: 'M', cls: 'bg-warn/15 text-warn', title: 'изменён' },
  D: { label: 'D', cls: 'bg-err/15 text-err', title: 'удалён' },
  A: { label: 'A', cls: 'bg-ok/15 text-ok', title: 'новый файл' },
  R: { label: 'R', cls: 'bg-brand/15 text-brand', title: 'переименован' },
};

const ROW_CLS: Record<string, string> = {
  add: 'bg-ok/10 text-ok',
  del: 'bg-err/10 text-err',
  ctx: 'text-tx-2',
  meta: 'text-tx-3',
};

/** Один изменённый файл: бейдж статуса + путь + счётчик строк; клик
 *  раскрывает ханки (Zed-стиль). Повторный клик сворачивает. */
export function GitDiffInline({ file, onOpenInEditor }: {
  file: GitDiffFile;
  onOpenInEditor?: (path: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const badge = STATUS_BADGE[file.status];
  const { added, deleted } = hunkStats(file.hunks);
  const openable = file.status !== 'D' || file.hunks.length > 0;

  return (
    <div className="rounded-md border border-bdr-s bg-bg-2 overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        title={file.path}
        className="w-full flex items-center gap-1.5 px-1.5 py-1 text-left hover:bg-bg-4/60 transition-colors cursor-pointer min-w-0"
      >
        <span
          className={clsx(
            'shrink-0 w-4 h-4 flex items-center justify-center rounded text-[10px] font-bold',
            badge.cls,
          )}
          title={badge.title}
        >
          {badge.label}
        </span>
        <span className="truncate flex-1 font-mono text-[11px] text-tx-2" dir="ltr">
          {file.path}
        </span>
        {(added > 0 || deleted > 0) && (
          <span className="shrink-0 font-mono text-[10px] tabular-nums">
            {added > 0 && <span className="text-ok">+{added}</span>}
            {added > 0 && deleted > 0 && <span className="text-tx-3"> </span>}
            {deleted > 0 && <span className="text-err">−{deleted}</span>}
          </span>
        )}
        <svg
          width="10"
          height="10"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={clsx('shrink-0 text-tx-3 transition-transform', expanded && 'rotate-90')}
        >
          <polyline points="6 4 10 8 6 12" />
        </svg>
      </button>

      {expanded && (
        <div className="border-t border-bdr-s bg-bg-1">
          {file.hunks.length === 0 ? (
            <div className="px-2 py-1.5 text-[11px] text-tx-3 italic">
              {file.status === 'A' ? 'Новый файл — ещё нет в репозитории' : 'Нет построчных изменений'}
            </div>
          ) : (
            <div className="max-h-56 overflow-y-auto py-0.5 font-mono text-[10px] leading-4">
              {file.hunks.flatMap((hunk, hi) =>
                parseHunk(hunk).map((row, ri) => (
                  <div
                    key={`${hi}-${ri}`}
                    className={clsx('flex px-1', ROW_CLS[row.kind])}
                  >
                    <span className="w-3.5 shrink-0 text-center select-none opacity-70">
                      {row.kind === 'add' ? '+' : row.kind === 'del' ? '−' : ' '}
                    </span>
                    <span className="whitespace-pre-wrap break-all min-w-0" dir="ltr">
                      {row.text || ' '}
                    </span>
                  </div>
                )),
              )}
            </div>
          )}
          {onOpenInEditor && openable && (
            <div className="px-1.5 pb-1.5 pt-0.5">
              <button
                type="button"
                onClick={() => onOpenInEditor(file.path)}
                className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md border border-bdr-s bg-bg-3 text-[10px] font-medium text-tx-2 hover:text-tx-1 hover:bg-bg-4 transition-colors cursor-pointer"
                title="Былой и рабочий варианты рядом"
              >
                <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 1h4v4" />
                  <path d="M15 1 9 7" />
                  <path d="M13 9v5a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5" />
                </svg>
                Открыть в редакторе
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
