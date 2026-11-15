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

/** Parses the AI model's streamed answer into (task, score, reason) triples. */
function parseAIAnswer(text: string): { task: string; score: number; reason: string }[] {
  text = text.replace(/<think>[\s\S]*?(?:<\/think>|$)/g, '').replace(/<!--\s*CF_[\s\S]*?(?:-->|$)/g, '');
  const start = text.indexOf('[');
  const end = text.lastIndexOf(']');
  if (start < 0 || end <= start) throw new Error('no json array');
  const arr = JSON.parse(text.slice(start, end + 1)) as Record<string, unknown>[];
  return arr
    .map((x) => ({
      task: typeof x.task === 'string' ? x.task : typeof x.task_slug === 'string' ? x.task_slug : '',
      score: typeof x.score === 'number' ? x.score : 0,
      reason: typeof x.reason === 'string' ? x.reason : '',
    }))
    .filter((x) => x.task !== '');
}

interface AIHit { entry: SearchCatalogEntry; score: number; reason: string }

export function SearchBar() {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [focused, setFocused] = useState(false);
  const [results, setResults] = useState<SearchTaskResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [catalogError, setCatalogError] = useState(false);
  const [aiEnabled, setAiEnabled] = useState(false);
  const [aiMode, setAiMode] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiNotice, setAiNotice] = useState('');
  const [aiHits, setAiHits] = useState<AIHit[] | null>(null);
  const [catalog, setCatalog] = useState<SearchCatalogEntry[] | null>(null);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const searchRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);

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

  // lazy data on first focus: AI availability + catalog for hot tags / AI mode
  useEffect(() => {
    if (!focused) return;
    api.aiConfig().then((c) => setAiEnabled(!!c.enabled)).catch(() => setAiEnabled(false));
    api.searchCatalog().then((r) => { setCatalog(r.tasks); setCatalogError(false); }).catch(() => setCatalogError(true));
    return () => abortRef.current?.abort();
  }, [focused]);

  const toggleAiMode = useCallback(() => {
    if (aiBusy) return;
    abortRef.current?.abort();
    setAiHits(null);
    setAiBusy(false);
    setAiNotice('');
    setAiMode((m) => !m);
    inputRef.current?.focus();
  }, [aiBusy]);

  const runAISearch = useCallback(
    async (q: string, ac: AbortController) => {
      setAiBusy(true);
      setAiNotice('');
      // catalog may still be loading when the user toggles AI quickly — fetch on demand
      let cat = catalog;
      if (!cat) {
        try {
          const r = await api.searchCatalog();
          cat = r.tasks;
          setCatalog(r.tasks);
        } catch {
          if (ac.signal.aborted) return;
          setAiBusy(false);
          setAiNotice('AI недоступен, показан обычный поиск');
          setAiHits(null);
          return;
        }
      }
      if (ac.signal.aborted) return;
      const summaryLines = cat
        .slice(0, 500)
        .map((t) => `- ${t.course}/${t.task} | ${t.title}${t.tags?.length ? ` | tags: ${t.tags.join(', ')}` : ''}`);
      const systemPrompt =
        'Ты — поисковик по задачам CourseForge. Тебе дан список задач (курс/слаг, заголовок, теги). ' +
        'Верни строго JSON-массив (без markdown) вида [{"task": "<курс>/<слаг>", "score": 0..1, "reason": "короткое пояснение"}] ' +
        'для не более чем 10 самых релевантных задач. Ничего кроме JSON не выводи.';
      let acc = '';
      let streamFailed = false;
      api.aiChatStream(
        [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: `Список задач:\n${summaryLines.join('\n')}\n\nЗапрос: ${q}` },
        ],
        (chunk) => { acc += chunk; },
        () => { streamFailed = true; },
        ac.signal,
        false,
      ).then(() => {
        if (ac.signal.aborted) return;
        setAiBusy(false);
        try {
          if (streamFailed) throw new Error('AI search failed');
          const parsed = parseAIAnswer(acc);
          const byKey = new Map<string, SearchCatalogEntry>(cat.map((t) => [`${t.course}/${t.task}`, t] as const));
          const hits: AIHit[] = parsed
            .map((p) => ({ entry: byKey.get(p.task), score: p.score, reason: p.reason }))
            .filter((h): h is AIHit => !!h.entry)
            .sort((a, b) => b.score - a.score);
          if (!hits.length) throw new Error('empty');
          setAiHits(hits);
        } catch {
          setAiNotice('AI недоступен, показан обычный поиск');
          setAiHits(null);
        }
      }).catch(() => {
        if (ac.signal.aborted) return;
        setAiBusy(false);
        setAiNotice('AI недоступен, показан обычный поиск');
        setAiHits(null);
      });
    },
    [catalog],
  );

  // debounced search (ordinary + AI mode)
  useEffect(() => {
    if (!focused || tokens.length === 0) return;
    const ac = new AbortController();
    abortRef.current = ac;
    const timer = setTimeout(() => {
      setLoading(true);
      setSearchError('');
      api.searchTasks(query.trim(), { limit: 50 })
        .then((r) => {
          if (ac.signal.aborted) return;
          setResults(r.results);
          if (!aiMode) setAiNotice('');
        })
        .catch(() => {
          if (ac.signal.aborted) return;
          setResults([]);
          setSearchError('Не удалось выполнить поиск. Измените запрос, чтобы повторить.');
        })
        .finally(() => { if (!ac.signal.aborted) setLoading(false); });
      if (aiMode) runAISearch(query.trim(), ac);
    }, DEBOUNCE_MS);
    return () => {
      ac.abort();
      clearTimeout(timer);
    };
  }, [query, aiMode, focused, tokens.length, runAISearch]);

  const flatEntries: { title: string; subtitle?: string; reason?: string; path: string; result?: SearchTaskResult }[] = useMemo(
    () => {
      if (aiMode && aiHits) {
        return aiHits.map((h) => ({
          title: h.entry.title,
          subtitle: `${h.entry.course}/${h.entry.task}`,
          reason: h.reason,
          path: `/courses/${h.entry.course}/tracks/${h.entry.track}/topics/${h.entry.topic}/units/${h.entry.unit}/tasks/${h.entry.task}`,
        }));
      }
      return (results ?? []).map((r) => ({ title: r.title, path: resultPath(r), result: r }));
    },
    [aiMode, aiHits, results],
  );

  // clamp the selection to the current list instead of resetting it via effect
  const activeIdx = Math.min(active, Math.max(flatEntries.length - 1, 0));

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
      setActive((a) => Math.min(Math.min(a + 1, flatEntries.length - 1), Math.max(flatEntries.length - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(Math.min(a - 1, flatEntries.length - 1), 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (loading || aiBusy || tokens.length === 0) return;
      const entry = flatEntries[activeIdx];
      if (entry) goTo(entry.path);
    }
  };

  // keep the active row visible
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${activeIdx}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [activeIdx]);

  const hasQuery = tokens.length > 0;
  const showAIList = aiMode && !aiBusy && aiHits !== null;
  const showEmptyHints = !hasQuery;

  return (
    <div
      ref={searchRef}
      className="contents"
      onFocus={() => setFocused(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) setFocused(false);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') { e.preventDefault(); close(); }
      }}
    >
      <div className="flex-1 min-w-0 flex justify-center px-2">
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
              setAiBusy(false);
              setAiHits(null);
              setAiNotice('');
            }}
            onKeyDown={onInputKeyDown}
            id="task-search-input"
            aria-label="Поиск задач"
            aria-autocomplete="list"
            aria-activedescendant={hasQuery && !loading && !aiBusy && flatEntries.length ? `search-result-${activeIdx}` : undefined}
            placeholder="Поиск задач"
            className="flex-1 min-w-0 bg-transparent outline-none text-base sm:text-sm text-tx-1 placeholder:text-tx-2 focus-visible:ring-0 focus-visible:outline-none"
            role="combobox"
            aria-expanded={open}
            aria-controls="search-results-list"
          />
          {query && (
            <button
              type="button"
              aria-label="Очистить поиск"
              onClick={() => { setQuery(''); setAiHits(null); setAiBusy(false); inputRef.current?.focus(); }}
              className="shrink-0 size-11 grid place-items-center rounded text-tx-2 hover:bg-bg-4 active:bg-bg-5 cursor-pointer"
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
            className="fixed inset-x-3 top-[52px] z-50 mx-auto flex max-h-[calc(100dvh-64px)] max-w-3xl flex-col overflow-hidden rounded-lg border border-bdr bg-bg-2 shadow-xl"
          >
            <div className="flex shrink-0 items-center justify-between gap-3 px-4 sm:px-5 min-h-12 border-b border-bdr">
              <label htmlFor="task-search-input" className="text-sm font-medium text-tx-1 cursor-pointer">Поиск задач</label>
              <div className="flex items-center gap-3">
                {aiEnabled && <button type="button" aria-pressed={aiMode} disabled={aiBusy} onClick={toggleAiMode} className="min-h-11 px-2 text-xs text-tx-2 rounded hover:bg-bg-4 active:bg-bg-5 disabled:cursor-not-allowed disabled:opacity-60 cursor-pointer">{aiMode ? 'AI-поиск включён' : 'Поиск с AI'}</button>}
                <button type="button" aria-label="Закрыть поиск" onClick={close} className="min-h-11 px-2 text-xs text-tx-2 rounded hover:bg-bg-4 active:bg-bg-5 cursor-pointer">Закрыть</button>
              </div>
            </div>
            {aiMode && <p className="shrink-0 px-4 sm:px-5 py-3 border-b border-bdr text-xs leading-5 text-tx-2">По смыслу названий и тегов.{catalog && catalog.length > 500 && ` В поиске первые 500 из ${catalog.length} задач.`}</p>}
            <div ref={listRef} id="search-results-list" role={hasQuery ? 'listbox' : undefined} aria-label={hasQuery ? 'Найденные задачи' : undefined} aria-busy={hasQuery && (loading || aiBusy)} className="min-h-0 max-h-[min(65vh,calc(100dvh-180px))] overflow-y-auto overscroll-contain">
              {showEmptyHints && (
                <div className="px-4 sm:px-5 py-6">
                  <p className="text-sm text-tx-1">Найдите задачу по названию, тегу или условию.</p>
                  <p className="text-xs leading-5 text-tx-2 mt-1">{aiMode ? 'Опишите тему или задайте вопрос своими словами.' : 'Все слова запроса должны встретиться в задаче.'}</p>
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

              {hasQuery && aiBusy && (
                <div role="status" className="px-5 py-8 text-sm text-tx-2">AI подбирает задачи…</div>
              )}

              {hasQuery && !aiBusy && showAIList && (
                <div className="py-1.5">
                  <div className="px-5 pt-2 pb-3 text-xs text-tx-2">Подобрано с AI</div>
                  {flatEntries.map((e, i) => (
                    <button
                      key={e.path}
                      type="button"
                      id={`search-result-${i}`}
                      data-idx={i}
                      role="option"
                      aria-selected={i === activeIdx}
                      onMouseEnter={() => setActive(i)}
                      onMouseDown={(ev) => ev.preventDefault()}
                      onClick={() => goTo(e.path)}
                      className={clsx(
                        'w-full text-left px-4 sm:px-5 py-3 border-t border-bdr-s cursor-pointer active:bg-bg-5',
                        i === activeIdx ? 'bg-bg-4' : 'hover:bg-bg-3/60',
                      )}
                    >
                      <div className="text-[15px] font-medium leading-6 text-tx-1 break-words">{e.title}</div>
                      <div className="text-xs text-tx-2 break-words mt-1">{e.subtitle}</div>
                      {e.reason && <div className="text-sm leading-5 text-tx-2 break-words mt-2">{e.reason}</div>}
                    </button>
                  ))}
                </div>
              )}

              {hasQuery && !aiBusy && !showAIList && (
                <div className="py-1.5">
                  {loading && <div role="status" className="px-5 py-8 text-sm text-tx-2">Ищем задачи…</div>}
                  {aiNotice && <div role="status" className="px-5 py-3 text-sm text-tx-2">{aiNotice}</div>}
                  {!loading && searchError && <div role="alert" className="px-5 py-6 text-sm text-tx-1">{searchError}</div>}
                  {!loading && !searchError && flatEntries.length > 0 && <div className="px-5 pt-2 pb-3 text-xs text-tx-2">{flatEntries.length === 50 ? 'Первые 50 результатов. Уточните запрос для более точного поиска.' : `Найдено: ${flatEntries.length}`}</div>}
                  {!loading && flatEntries.map((e, i) => {
                    const r = e.result!;
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
                        onClick={() => goTo(e.path)}
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
                  {!loading && !searchError && hasQuery && flatEntries.length === 0 && (
                    <div className="px-4 sm:px-5 py-8 break-words">
                      <p className="text-base font-medium text-tx-1">Ничего не найдено по «{query.trim()}»</p>
                      <p className="text-sm leading-6 text-tx-2 mt-2">Сократите запрос или попробуйте другое название темы.</p>
                      {aiEnabled && !aiMode && (
                        <button type="button" onClick={toggleAiMode} className="mt-3 min-h-11 px-3 border border-bdr rounded text-sm text-tx-1 hover:bg-bg-4 active:bg-bg-5 cursor-pointer">Попробовать поиск с AI</button>
                      )}
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
