import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AnimatePresence, motion } from 'framer-motion';
import clsx from 'clsx';
import { api } from '../api/client';
import type { SearchCatalogEntry, SearchTaskResult } from '../api/types';
import { TagBadge } from './ui/TagBadge';
import { DifficultyBadge } from './ui/DifficultyBadge';

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
          <mark key={i} className="bg-warn/30 text-inherit rounded-sm px-0.5">{p.s}</mark>
        ) : (
          <span key={i}>{p.s}</span>
        ),
      )}
    </>
  );
}

/** Parses the AI model's streamed answer into (task, score, reason) triples. */
function parseAIAnswer(text: string): { task: string; score: number; reason: string }[] {
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

/**
 * Header-integrated search: a wide input lives right in the top bar; typing
 * dims the page below the header and reveals a full-width results list.
 */
export function SearchBar() {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [focused, setFocused] = useState(false);
  const [results, setResults] = useState<SearchTaskResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [aiEnabled, setAiEnabled] = useState(false);
  const [aiMode, setAiMode] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiNotice, setAiNotice] = useState('');
  const [aiHits, setAiHits] = useState<AIHit[] | null>(null);
  const [catalog, setCatalog] = useState<SearchCatalogEntry[] | null>(null);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
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
    api.searchCatalog().then((r) => setCatalog(r.tasks)).catch(() => setCatalog(null));
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
    (q: string) => {
      if (!catalog) return;
      setAiBusy(true);
      setAiNotice('');
      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;
      const summaryLines = catalog
        .slice(0, 500)
        .map((t) => `- ${t.course}/${t.task} | ${t.title}${t.tags?.length ? ` | tags: ${t.tags.join(', ')}` : ''}`);
      const systemPrompt =
        'Ты — поисковик по задачам CourseForge. Тебе дан список задач (курс/слаг, заголовок, теги). ' +
        'Верни строго JSON-массив (без markdown) вида [{"task": "<курс>/<слаг>", "score": 0..1, "reason": "короткое пояснение"}] ' +
        'для не более чем 10 самых релевантных задач. Ничего кроме JSON не выводи.';
      let acc = '';
      api.aiChatStream(
        [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: `Список задач:\n${summaryLines.join('\n')}\n\nЗапрос: ${q}` },
        ],
        (chunk) => { acc += chunk; },
        () => setAiNotice('AI недоступен, показан обычный поиск'),
        ac.signal,
      ).then(() => {
        setAiBusy(false);
        if (ac.signal.aborted) return;
        try {
          const parsed = parseAIAnswer(acc);
          const byKey = new Map<string, SearchCatalogEntry>(catalog.map((t) => [`${t.course}/${t.task}`, t] as const));
          const hits: AIHit[] = parsed
            .map((p) => ({ entry: byKey.get(p.task), score: p.score, reason: p.reason }))
            .filter((h): h is AIHit => !!h.entry)
            .sort((a, b) => b.score - a.score);
          if (!hits.length) throw new Error('empty');
          setAiHits(hits);
        } catch {
          setAiNotice('AI недоступен, показан обычный поиск');
          setAiHits([]);
        }
      });
    },
    [catalog],
  );

  // debounced search (ordinary + AI mode)
  useEffect(() => {
    if (!focused || tokens.length === 0) return;
    const ac = new AbortController();
    const timer = setTimeout(() => {
      setLoading(true);
      if (aiMode) {
        // AI mode: keep ordinary results proximate for the fallback banner
        api.searchTasks(query.trim(), { limit: 50 })
          .then((r) => { if (!ac.signal.aborted) setResults(r.results); })
          .catch(() => { if (!ac.signal.aborted) setResults([]); })
          .finally(() => { if (!ac.signal.aborted) setLoading(false); });
        runAISearch(query.trim());
      } else {
        api.searchTasks(query.trim(), { limit: 50 })
          .then((r) => {
            if (ac.signal.aborted) return;
            setResults(r.results);
            setAiNotice('');
          })
          .catch(() => { if (!ac.signal.aborted) setResults([]); })
          .finally(() => { if (!ac.signal.aborted) setLoading(false); });
      }
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
    inputRef.current?.blur();
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
      if (query) {
        setQuery('');
      } else {
        close();
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((a) => Math.min(Math.min(a + 1, flatEntries.length - 1), Math.max(flatEntries.length - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(Math.min(a - 1, flatEntries.length - 1), 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
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
  const showAIList = aiMode && !aiBusy && !!aiHits;
  const showEmptyHints = !hasQuery;

  return (
    <>
      {/* search input is part of the header's flex row */}
      <div className="flex-1 min-w-0 flex justify-center px-2">
        <div
          className={clsx(
            'flex items-center gap-2 w-full max-w-3xl h-8 px-3 rounded-lg border transition-colors',
            focused
              ? 'bg-bg-1 border-brand/40 shadow-sm'
              : 'bg-bg-3/60 border-bdr-s hover:border-bdr',
          )}
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-tx-3 shrink-0">
            <circle cx="11" cy="11" r="7" />
            <line x1="21" y1="21" x2="16.5" y2="16.5" />
          </svg>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            onKeyDown={onInputKeyDown}
            placeholder="Поиск задач: слова, теги или вопрос…"
            className="flex-1 min-w-0 bg-transparent outline-none text-[15px] text-tx-1 placeholder:text-tx-3/80"
            role="combobox"
            aria-expanded={open}
            aria-controls="search-results-list"
          />
          {aiEnabled && (
            <button
              type="button"
              onMouseDown={(e) => e.preventDefault()} // keep input focus
              onClick={toggleAiMode}
              className={clsx(
                'shrink-0 inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium border transition-colors cursor-pointer',
                aiMode ? 'bg-brand/15 text-brand border-brand/35' : 'bg-bg-3 text-tx-3 border-bdr-s hover:text-tx-2',
              )}
              title="Умный поиск (AI подбирает релевантные задачи по смыслу запроса)"
            >
              ✦ Умный
            </button>
          )}
          {!aiEnabled && (
            <kbd className="shrink-0 hidden sm:inline text-[10px] text-tx-3 border border-bdr-s rounded px-1 py-0.5 bg-bg-2">
              Ctrl+K
            </kbd>
          )}
        </div>
      </div>

      {/* dim + full-width results beam below the header */}
      <AnimatePresence>
        {open && (
          <motion.div
            key="search-beam"
            className="fixed inset-x-0 top-11 bottom-0 z-40 bg-black/40"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            onMouseDown={(e) => {
              e.preventDefault();
              close();
            }}
          />
        )}
      </AnimatePresence>
      <AnimatePresence>
        {open && (
          <motion.div
            key="search-panel"
            className="fixed inset-x-0 top-11 z-40 border-b border-bdr bg-bg-2 shadow-[0_12px_32px_rgba(0,0,0,0.25)]"
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.16 }}
          >
            <div ref={listRef} id="search-results-list" role="listbox" className="max-h-[60vh] overflow-y-auto">
              {showEmptyHints && (
                <div className="px-6 py-4">
                  <div className="text-xs text-tx-3 mb-2.5">Начните печатать… или выберите горячий тег:</div>
                  <div className="flex flex-wrap gap-1.5">
                    {hotTags.map((tag) => (
                      <TagBadge
                        key={tag}
                        tag={tag}
                        size="sm"
                        onClick={() => {
                          setQuery(tag);
                          inputRef.current?.focus();
                        }}
                      />
                    ))}
                    {!hotTags.length && <span className="text-xs text-tx-3">Каталог задач ещё не загружен</span>}
                  </div>
                </div>
              )}

              {hasQuery && aiBusy && (
                <div className="px-6 py-6 text-sm text-tx-3 animate-pulse">AI подбирает задачи…</div>
              )}

              {hasQuery && !aiBusy && showAIList && (
                <div className="py-1.5">
                  <div className="px-6 pt-1.5 pb-1 text-[11px] uppercase tracking-wide text-tx-3">Нашёл AI</div>
                  {flatEntries.map((e, i) => (
                    <button
                      key={e.path}
                      type="button"
                      data-idx={i}
                      onMouseEnter={() => setActive(i)}
                      onMouseDown={(ev) => ev.preventDefault()}
                      onClick={() => goTo(e.path)}
                      className={clsx(
                        'w-full text-left px-6 py-2.5 transition-colors cursor-pointer',
                        i === activeIdx ? 'bg-bg-4' : 'hover:bg-bg-3/60',
                      )}
                    >
                      <div className="text-sm text-tx-1 truncate">{e.title}</div>
                      <div className="text-xs text-tx-3 truncate mt-0.5">{e.subtitle}</div>
                      {e.reason && <div className="text-xs text-tx-2 truncate mt-0.5">{e.reason}</div>}
                    </button>
                  ))}
                </div>
              )}

              {hasQuery && !aiBusy && !showAIList && (
                <div className="py-1.5">
                  {loading && <div className="px-6 py-1.5 text-xs text-tx-3">Поиск…</div>}
                  {aiNotice && <div className="px-6 py-1.5 text-xs text-warn">{aiNotice}</div>}
                  {!loading && flatEntries.map((e, i) => {
                    const r = e.result!;
                    return (
                      <button
                        key={`${r.course}-${r.task}-${i}`}
                        type="button"
                        data-idx={i}
                        role="option"
                        aria-selected={i === activeIdx}
                        onMouseEnter={() => setActive(i)}
                        onMouseDown={(ev) => ev.preventDefault()}
                        onClick={() => goTo(e.path)}
                        className={clsx(
                          'w-full text-left px-6 py-2.5 transition-colors cursor-pointer',
                          i === activeIdx ? 'bg-bg-4' : 'hover:bg-bg-3/60',
                        )}
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <span className="text-sm text-tx-1 truncate">
                            <Highlighted text={r.title} tokens={tokens} />
                          </span>
                          <span className="ml-auto flex items-center gap-2 shrink-0">
                            <span className="text-[11px] text-tx-3">{r.course} › {r.topic}</span>
                            <DifficultyBadge difficulty={r.difficulty} size="sm" />
                            {r.rank === 'tag' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded bg-brand/10 text-brand border border-brand/25">тег</span>
                            )}
                            {r.rank === 'content' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded bg-bg-3 text-tx-3 border border-bdr-s">условие</span>
                            )}
                          </span>
                        </div>
                        {r.snippet && (
                          <div className="text-xs text-tx-3 truncate mt-0.5">
                            <Highlighted text={r.snippet} tokens={tokens} />
                          </div>
                        )}
                        {r.tags && r.tags.length > 0 && (
                          <div className="flex items-center gap-1.5 mt-1 flex-wrap">
                            {r.tags.slice(0, 6).map((tag) => (
                              <TagBadge key={tag} tag={tag} size="sm" />
                            ))}
                          </div>
                        )}
                      </button>
                    );
                  })}
                  {!loading && hasQuery && flatEntries.length === 0 && (
                    <div className="px-6 py-6 text-sm text-tx-3">
                      Ничего не найдено по «{query.trim()}»
                      {aiEnabled && !aiMode && (
                        <> — попробуйте <button type="button" onMouseDown={(ev) => ev.preventDefault()} onClick={toggleAiMode} className="text-brand hover:underline cursor-pointer">умный поиск ✦</button></>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* footer hints */}
            <div className="flex items-center gap-4 px-6 h-8 border-t border-bdr text-[11px] text-tx-3">
              <span>↑↓ — навигация</span>
              <span>Enter — перейти</span>
              <span>Esc — закрыть</span>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
