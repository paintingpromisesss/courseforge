import { useEffect, useRef, useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../../api/client';
import type { RunnerStatus } from '../../api/types';

const LANG_LABELS: Record<string, string> = {
  go: 'Go', golang: 'Go',
  python: 'Python', python3: 'Python', py: 'Python',
  cpp: 'C++', 'c++': 'C++',
  csharp: 'C#', cs: 'C#',
  java: 'Java',
  javascript: 'JavaScript', js: 'JavaScript',
  typescript: 'TypeScript', ts: 'TypeScript',
  rust: 'Rust', rs: 'Rust',
};

function label(lang: string): string {
  return LANG_LABELS[lang.toLowerCase()] || lang;
}

function ChevronIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

// Warning shown when a language has no working runner installed.
function WarnIcon({ className }: { className?: string }) {
  return (
    <svg className={className} width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  );
}

interface Props {
  languages: string[];
  value: string;
  onChange: (lang: string) => void;
}

export function LangSelect({ languages, value, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // Reuse SettingsPanel's detect cache: status drives the "!" badge, version the subtitle.
  const results = useQueries({
    queries: languages.map((l) => ({
      queryKey: ['runner-detect', l],
      queryFn: () => api.detectRunner(l),
      staleTime: Infinity,
      refetchOnWindowFocus: false,
    })),
  });
  const statusOf = (lang: string): RunnerStatus | undefined =>
    results[languages.indexOf(lang)]?.data;

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const notInstalled = (st?: RunnerStatus) => !!st && st.status !== 'ok';
  const current = statusOf(value);

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1.5 rounded-md border border-bdr bg-bg-3 px-2.5 py-1 text-sm text-tx-1 transition-colors hover:bg-bg-4"
      >
        {notInstalled(current) && <WarnIcon className="text-warn" />}
        <span>{label(value)}</span>
        <span className="text-tx-3"><ChevronIcon /></span>
      </button>

      {open && (
        <div className="absolute left-0 z-40 mt-1 min-w-[220px] overflow-hidden rounded-lg border border-bdr bg-bg-2 py-1 shadow-xl">
          {languages.map((l) => {
            const st = statusOf(l);
            const selected = l === value;
            return (
              <button
                key={l}
                type="button"
                onClick={() => {
                  onChange(l);
                  setOpen(false);
                }}
                className={clsx(
                  'flex w-full items-center gap-2 px-3 py-2 text-left transition-colors',
                  selected ? 'bg-bg-4' : 'hover:bg-bg-3',
                )}
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5 text-sm text-tx-1">
                    <span>{label(l)}</span>
                    {notInstalled(st) && <WarnIcon className="text-warn shrink-0" />}
                  </div>
                  {st?.version && <div className="truncate text-xs text-tx-3">{st.version}</div>}
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
