import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { shortCommit } from '../lib/gitHelpers';
import { relativeTimeRu } from './gitTime';

/** История коммитов: хэш, заголовок, человекочитаемое время; клик
 *  раскрывает аккордеон со списком файлов коммита. */
export function GitHistoryList({ courseSlug }: { courseSlug: string }) {
  const [expanded, setExpanded] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['gitLog', courseSlug],
    queryFn: () => api.gitLog(courseSlug, 30),
    retry: false,
  });

  // файлы раскрываемого коммита — один аккордеон за раз
  const { data: filesData, isFetching: filesLoading } = useQuery({
    queryKey: ['gitCommitFiles', courseSlug, expanded],
    queryFn: () => api.gitCommitFiles(courseSlug, expanded!),
    enabled: !!expanded,
    staleTime: 5 * 60_000,
  });

  if (isLoading) {
    return <div className="px-1 py-1 text-[11px] text-tx-3">Загрузка истории...</div>;
  }

  const commits = data?.commits ?? [];
  if (commits.length === 0) {
    return <div className="px-1 py-1 text-[11px] text-tx-3">История пуста</div>;
  }

  return (
    <div className="space-y-0.5">
      {commits.map((c) => {
        const isOpen = expanded === c.hash;
        return (
          <div key={c.hash}>
            <button
              type="button"
              onClick={() => setExpanded(isOpen ? null : c.hash)}
              title={`${shortCommit(c.hash)} — ${c.author}`}
              className={clsx(
                'w-full flex items-center gap-1.5 px-1.5 py-1 rounded text-left transition-colors cursor-pointer min-w-0',
                isOpen ? 'bg-bg-4 text-tx-1' : 'text-tx-2 hover:bg-bg-4/60 hover:text-tx-1',
              )}
            >
              <span className="shrink-0 font-mono text-[10px] text-tx-3" dir="ltr">
                {shortCommit(c.hash)}
              </span>
              <span className="truncate flex-1 text-[11px]">{c.subject}</span>
              <span className="shrink-0 text-[10px] text-tx-3">
                {relativeTimeRu(c.time)}
              </span>
            </button>

            {isOpen && (
              <div className="ml-3 mt-0.5 mb-1 pl-2 border-l border-bdr-s space-y-0.5">
                {filesLoading && (
                  <div className="py-0.5 text-[10px] text-tx-3">Загрузка файлов...</div>
                )}
                {!filesLoading && (filesData?.files ?? []).length === 0 && (
                  <div className="py-0.5 text-[10px] text-tx-3">Нет файлов</div>
                )}
                {(filesData?.files ?? []).map((f) => (
                  <div key={f} className="flex items-center gap-1.5 py-0.5 min-w-0" title={f}>
                    <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-tx-3">
                      <path d="M13 5 8 1H3a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1V5z" />
                      <polyline points="8 1 8 5 13 5" />
                    </svg>
                    <span className="truncate font-mono text-[10px] text-tx-2" dir="ltr">{f}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
