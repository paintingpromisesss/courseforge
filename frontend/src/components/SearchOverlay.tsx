import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import clsx from 'clsx';
import { api } from '../api/client';
import type { SearchCatalogEntry, SearchTaskResult } from '../api/types';
import { getDifficultyConfig } from './ui/DifficultyBadge';

const DEBOUNCE_MS = 250;

function resultPath(r: SearchTaskResult) {
  return `/courses/${r.course}/tracks/${r.track}/topics/${r.topic}/units/${r.unit}/tasks/${r.task}`;
}

/** Splits text into plain/highlight segments for the given query tokens. */
function highlight(text: string, tokens: string[]): { s: string; hit: boolean }[] {
  if (!tokens.length || !text) return [{ s: text, hit: false }];
  const lower = text.toLowerCase();
  const marks = new Array<boolean>(text.length).fill(false);
  for (const t of tokens) {
    let from = 0;
    while (from <= lower.length - t.length) {
      const i = lower.indexOf(t, from);
      if (i < 0) break;
      for (let j = i; j < i + t.length; j++) marks[j] = true;
      from = i + t.length;
    }
  }
  const parts: { s: string; hit: boolean }[] = [];
  let cur = '';
  let curHit = marks[0] ?? false;
  for (let i = 0; i < text.length; i++) {
    if (marks[i] === curHit) {
      cur += text[i];
    } else {
      if (cur) parts.push({ s: cur, hit: curHit });
      cur = text[i];
      curHit = marks[i];
    }
  }
  if (cur) parts.push({ s: cur, hit: curHit });
  return parts;
}

function Highlighted({ text, tokens }: { text: string; tokens: string[] }) {
  return (
    <>
      {highlight(text, tokens).map((p, i) =>
        p.hit ? (
          <mark key={i} className="bg-transparent text-inherit underline decoration-brand decoration-2 underline-offset-2">{p.s}</mark>
        ) : (
          <span key={i}>{p.s}</span>
        ),
      )}
    </>
  );
}

export function SearchBar() {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [focused, setFocused] = useState(false);
  const [results, setResults] = useState<SearchTaskResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [catalogError, setCatalogError] = useState(false);
  const [catalog, setCatalog] = useState<SearchCatalogEntry[] | null>(null);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const searchRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const tokens = useMemo(
    () => query.trim().toLowerCase().split(/\s+/).filter(Boolean),
    [query],
  );
  const open = focused;

  // Ctrl+K focuses the header input from anywhere
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        inputRef.current?.focus();
        inputRef.current?.select();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  useEffect(() => {
    if (!focused) return;
    api.searchCatalog().then((r) => { setCatalog(r.tasks); setCatalogError(false); }).catch(() => setCatalogError(true));
  }, [focused]);

  useEffect(() => {
    if (tokens.length === 0) return;
    const ac = new AbortController();
    const timer = setTimeout(() => {
      setLoading(true);
      setSearchError('');
      api.searchTasks(query.trim(), { limit: 50 })
        .then((r) => {
          if (ac.signal.aborted) return;
          setResults(r.results);
        })
        .catch(() => {
          if (ac.signal.aborted) return;
          setResults([]);
          setSearchError('Не удалось выполнить поиск. Измените запрос, чтобы повторить.');
        })
        .finally(() => { if (!ac.signal.aborted) setLoading(false); });
    }, DEBOUNCE_MS);
    return () => {
      ac.abort();
      clearTimeout(timer);
    };
  }, [query, tokens.length]);

  // clamp the selection to the current list instead of resetting it via effect
  const activeIdx = Math.min(active, Math.max((results?.length ?? 0) - 1, 0));

  const hotTags = useMemo(() => {
    if (!catalog || tokens.length > 0) return [];
    const counts = new Map<string, number>();
    for (const t of catalog) for (const tag of t.tags ?? []) counts.set(tag, (counts.get(tag) ?? 0) + 1);
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 14).map(([tag]) => tag);
  }, [catalog, tokens.length]);

  const close = useCallback(() => {
    setFocused(false);
    const focusedElement = document.activeElement;
    if (focusedElement instanceof HTMLElement && searchRef.current?.contains(focusedElement)) {
      focusedElement.blur();
    }
  }, []);

  const goTo = useCallback(
    (path: string) => {
      close();
      navigate(path);
    },
    [navigate, close],
  );

  const onInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, Math.max((results?.length ?? 0) - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (loading || tokens.length === 0) return;
      const result = results?.[activeIdx];
      if (result) goTo(resultPath(result));
    }
  };

  // keep the active row visible
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${activeIdx}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [activeIdx]);

  const hasQuery = tokens.length > 0;
  const showEmptyHints = !hasQuery;

  return (
    <div
      ref={searchRef}
      className="relative mx-auto w-full min-w-0 max-w-3xl flex-1"
      onFocus={() => setFocused(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) setFocused(false);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') { e.preventDefault(); close(); }
      }}
    >
        <div
          className={clsx(
            'flex items-center gap-2 w-full max-w-3xl h-8 px-3 rounded-md border',
            focused
              ? 'max-sm:fixed max-sm:inset-x-3 max-sm:top-0 max-sm:z-50 max-sm:w-auto max-sm:h-11 bg-bg-1 border-brand'
              : 'bg-bg-3/60 border-bdr-s hover:border-bdr',
          )}
        >
          <svg aria-hidden="true" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-tx-2 shrink-0">
            <circle cx="11" cy="11" r="7" />
            <line x1="21" y1="21" x2="16.5" y2="16.5" />
          </svg>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
              setLoading(true);
            }}
            onKeyDown={onInputKeyDown}
            id="task-search-input"
            aria-label="Поиск задач"
            aria-autocomplete="list"
            aria-activedescendant={open && hasQuery && !loading && results?.length ? `search-result-${activeIdx}` : undefined}
            placeholder="Поиск задач"
            className="flex-1 min-w-0 bg-transparent outline-none text-base sm:text-sm text-tx-1 placeholder:text-tx-2 focus-visible:ring-0 focus-visible:outline-none"
            role="combobox"
            aria-expanded={open}
            aria-controls="search-results-list"
          />
          {open && query && (
            <button
              type="button"
              aria-label="Очистить поиск"
              onClick={() => { setQuery(''); inputRef.current?.focus(); }}
              className="shrink-0 size-7 grid place-items-center rounded text-tx-2 hover:bg-bg-4 active:bg-bg-5 cursor-pointer"
            >
              <svg aria-hidden="true" width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="m4 4 8 8M12 4l-8 8" /></svg>
            </button>
          )}
          {!focused && (
            <kbd className="shrink-0 hidden sm:inline text-[10px] text-tx-2 border border-bdr-s rounded px-1 py-0.5 bg-bg-2">
              Ctrl+K
            </kbd>
          )}
        </div>
      {open && (
        <>
          <div
            className="fixed inset-x-0 top-11 bottom-0 z-40 bg-black/40"
            onMouseDown={(e) => {
              e.preventDefault();
              close();
            }}
          />
          <div
            className="fixed inset-x-3 top-[52px] z-50 flex max-h-[calc(100dvh-64px)] flex-col overflow-hidden rounded-lg border border-bdr bg-bg-2 shadow-xl sm:absolute sm:inset-x-0 sm:top-[calc(100%+14px)]"
          >
            <div className="flex shrink-0 items-center justify-between gap-3 px-4 sm:px-5 min-h-12 border-b border-bdr">
              <label htmlFor="task-search-input" className="text-sm font-medium text-tx-1 cursor-pointer">Поиск задач</label>
              <div className="flex items-center gap-3">
                <button type="button" aria-label="Закрыть поиск" onClick={close} className="min-h-11 px-2 text-xs text-tx-2 rounded hover:bg-bg-4 active:bg-bg-5 cursor-pointer">Закрыть</button>
              </div>
            </div>
            <div ref={listRef} id="search-results-list" role={hasQuery ? 'listbox' : undefined} aria-label={hasQuery ? 'Найденные задачи' : undefined} aria-busy={hasQuery && loading} className="min-h-0 max-h-[min(65vh,calc(100dvh-180px))] overflow-y-auto overscroll-contain">
              {showEmptyHints && (
                <div className="px-4 sm:px-5 py-6">
                  <p className="text-sm text-tx-1">Найдите задачу по названию, тегу или условию.</p>
                  <p className="text-xs leading-5 text-tx-2 mt-1">Все слова запроса должны встретиться в задаче.</p>
                  {hotTags.length > 0 && <div className="text-xs text-tx-2 mt-5 mb-2">Частые теги</div>}
                  <div className="flex flex-wrap gap-2">
                    {hotTags.map((tag) => (
                      <button
                        key={tag}
                        type="button"
                        className="min-h-11 px-3 rounded border border-bdr text-sm text-tx-1 hover:bg-bg-4 active:bg-bg-5 cursor-pointer"
                        onClick={() => {
                          setQuery(tag);
                          setLoading(true);
                          setActive(0);
                          inputRef.current?.focus();
                        }}
                      >{tag}</button>
                    ))}
                    {catalogError && <span className="text-xs text-tx-2 mt-4">Не удалось загрузить теги. Поиск по словам доступен.</span>}
                    {!catalog && !catalogError && <span className="text-xs text-tx-2 mt-4">Загружаем теги…</span>}
                    {catalog?.length === 0 && <span className="text-xs text-tx-2 mt-4">В библиотеке пока нет задач. Добавьте курс с задачами.</span>}
                  </div>
                </div>
              )}

              {hasQuery && (
                <div className="py-1.5">
                  {loading && <div role="status" className="px-5 py-8 text-sm text-tx-2">Ищем задачи…</div>}
                  {!loading && searchError && <div role="alert" className="px-5 py-6 text-sm text-tx-1">{searchError}</div>}
                  {!loading && !searchError && !!results?.length && <div className="px-5 pt-2 pb-3 text-xs text-tx-2">{results.length === 50 ? 'Первые 50 результатов. Уточните запрос для более точного поиска.' : `Найдено: ${results.length}`}</div>}
                  {!loading && results?.map((r, i) => {
                    return (
                      <button
                        key={`${r.course}-${r.task}-${i}`}
                        type="button"
                        id={`search-result-${i}`}
                        data-idx={i}
                        role="option"
                        aria-selected={i === activeIdx}
                        onMouseEnter={() => setActive(i)}
                        onMouseDown={(ev) => ev.preventDefault()}
                        onClick={() => goTo(resultPath(r))}
                        className={clsx(
                          'w-full text-left px-4 sm:px-5 py-3 border-t border-bdr-s cursor-pointer active:bg-bg-5',
                          i === activeIdx ? 'bg-bg-4' : 'hover:bg-bg-3/60',
                        )}
                      >
                        <div className="text-[15px] leading-6 font-medium text-tx-1 break-words">
                          <Highlighted text={r.title} tokens={tokens} />
                        </div>
                        <div className="text-xs leading-5 text-tx-2 mt-1 break-words">{r.course} <span aria-hidden="true">/</span> {r.topic}{!!r.difficulty && ` · ${getDifficultyConfig(r.difficulty).label}`}</div>
                        {r.snippet && (
                          <div className="text-sm leading-5 text-tx-2 break-words mt-2 line-clamp-2">
                            <Highlighted text={r.snippet} tokens={tokens} />
                          </div>
                        )}
                        {r.tags && r.tags.length > 0 && (
                          <div className="text-xs leading-5 text-tx-2 break-words mt-1">{r.tags.slice(0, 6).join(' · ')}</div>
                        )}
                      </button>
                    );
                  })}
                  {!loading && !searchError && hasQuery && results?.length === 0 && (
                    <div className="px-4 sm:px-5 py-8 break-words">
                      <p className="text-base font-medium text-tx-1">Ничего не найдено по «{query.trim()}»</p>
                      <p className="text-sm leading-6 text-tx-2 mt-2">Сократите запрос или попробуйте другое название темы.</p>
                    </div>
                  )}
                </div>
              )}
            </div>

            <div className="hidden sm:flex shrink-0 items-center gap-5 px-5 h-10 border-t border-bdr text-xs text-tx-2">
              <span><kbd>↑ ↓</kbd> выбрать</span>
              <span><kbd>Enter</kbd> открыть</span>
              <span className="ml-auto"><kbd>Esc</kbd> закрыть</span>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
