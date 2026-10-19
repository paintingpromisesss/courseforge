import { useEffect, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { computeSplitDiff } from './lineDiff';

/** Полноценный дифф «было — стало»: левая колонка — версия из HEAD,
 *  правая — рабочая версия. Строки выровнены попарно (единый скролл),
 *  read-only. Открывается из инлайн-диффа git-дока. */
export function GitDiffSplit({ path, onClose }: { path: string; onClose: () => void }) {
  const { courseSlug } = useParams<{ courseSlug: string }>();

  const { data: oldText, isError: oldError } = useQuery({
    queryKey: ['gitFile', courseSlug, 'HEAD', path],
    queryFn: () => api.gitFile(courseSlug!, path, 'HEAD'),
    // новый файл: в HEAD его нет (404) — справа «стало», слева пусто
    retry: false,
    staleTime: 60_000,
  });
  const { data: newText, isError: newError } = useQuery({
    queryKey: ['gitFile', courseSlug, 'worktree', path],
    queryFn: () => api.gitFile(courseSlug!, path),
    // удалённый файл: на диске его больше нет — слева «было», справа пусто
    retry: false,
    staleTime: 60_000,
  });

  // Esc закрывает просмотр
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [onClose]);

  const rows = useMemo(
    () => computeSplitDiff(oldError ? '' : oldText ?? '', newError ? '' : newText ?? ''),
    [oldText, oldError, newText, newError],
  );
  const changed = rows.filter((r) => r.kind !== 'eq').length;
  const bothFailed = oldError && newError;

  return (
    <div className="h-full flex flex-col bg-bg-1">
      {/* шапка: путь, объём изменений, закрытие */}
      <div className="h-11 shrink-0 px-3 border-b border-bdr bg-bg-2 flex items-center gap-3 select-none">
        <span className="text-xs font-semibold uppercase tracking-wider text-tx-3">Дифф</span>
        <span className="truncate font-mono text-xs text-tx-1 flex-1" dir="ltr" title={path}>
          {path}
        </span>
        {changed > 0 && (
          <span className="shrink-0 text-[11px] text-tx-3 tabular-nums">
            {changed} стр. различаются
          </span>
        )}
        <button
          type="button"
          onClick={onClose}
          title="Закрыть дифф (Esc)"
          className="shrink-0 w-7 h-7 flex items-center justify-center rounded-md text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
        >
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      {bothFailed ? (
        <div className="flex-1 flex items-center justify-center text-sm text-tx-3">
          Не удалось прочитать файл ни в HEAD, ни на диске
        </div>
      ) : (!oldText && !oldError) || (!newText && !newError) ? (
        <div className="flex-1 flex items-center justify-center text-sm text-tx-3">Загрузка...</div>
      ) : (
        <div className="flex-1 overflow-auto font-mono text-[11px] leading-5">
          {/* заголовки колонок */}
          <div className="sticky top-0 z-10 grid grid-cols-2 bg-bg-2 border-b border-bdr">
            <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-tx-3 border-r border-bdr">
              Было · HEAD
            </div>
            <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-tx-3">
              Стало · рабочая версия
            </div>
          </div>

          {rows.map((row, idx) => {
            const left = row.kind === 'add' ? null : { n: row.n, text: row.text };
            const right = row.kind === 'del' ? null : { n: row.n, text: row.text };
            return (
              <div key={idx} className="grid grid-cols-2 min-w-0">
                {/* левая колонка («было») */}
                <div
                  className={clsx(
                    'flex items-start px-3 min-w-0 border-r border-bdr-s/60',
                    row.kind === 'del' && 'bg-err/10',
                    row.kind === 'add' && 'bg-bg-2/40',
                  )}
                >
                  {left ? (
                    <>
                      <span className={clsx('w-8 shrink-0 text-right pr-2 select-none tabular-nums', row.kind === 'del' ? 'text-err/60' : 'text-tx-3/50')}>
                        {left.n}
                      </span>
                      <span className={clsx('whitespace-pre-wrap break-all min-w-0', row.kind === 'del' ? 'text-err' : 'text-tx-2')}>
                        {left.text || ' '}
                      </span>
                    </>
                  ) : (
                    <span className="w-8" />
                  )}
                </div>
                {/* правая колонка («стало») */}
                <div
                  className={clsx(
                    'flex items-start px-3 min-w-0',
                    row.kind === 'add' && 'bg-ok/10',
                    row.kind === 'del' && 'bg-bg-2/40',
                  )}
                >
                  {right ? (
                    <>
                      <span className={clsx('w-8 shrink-0 text-right pr-2 select-none tabular-nums', row.kind === 'add' ? 'text-ok/60' : 'text-tx-3/50')}>
                        {right.n}
                      </span>
                      <span className={clsx('whitespace-pre-wrap break-all min-w-0', row.kind === 'add' ? 'text-ok' : 'text-tx-2')}>
                        {right.text || ' '}
                      </span>
                    </>
                  ) : (
                    <span className="w-8" />
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
