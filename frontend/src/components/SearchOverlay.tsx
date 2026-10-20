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

export function SearchOverlay({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
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

  const tokens = query.trim().toLowerCase().split(/\s+/).filter(Boolean).length
    ? query.trim().toLowerCase().split(/\s+/).filter(Boolean)
    : [];

  // lazy data on first open: AI availability + catalog for hot tags / AI mode
  useEffect(() => {
    if (!open) return;
    api.aiConfig().then((c) => setAiEnabled(!!c.enabled)).catch(() => setAiEnabled(false));
    api.searchCatalog().then((r) => setCatalog(r.tasks)).catch(() => setCatalog(null));
    return () => abortRef.current?.abort();
  }, [open]);

  // focus input on open
  useEffect(() => {
    if (open) setTimeout(() => inputRef.current?.focus(), 30);
  }, [open]);

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

  // debounced search (ordinary + AI fallback results)
  useEffect(() => {
    if (!open || tokens.length === 0) return;
    const ac = new AbortController();
    const timer = setTimeout(() => {
      setLoading(true);
      if (aiMode) {
        // AI mode: fetch ordinary results for the fallback banner and run AI
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
  }, [query, aiMode, open, tokens.length, runAISearch]);

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

  const hotTags = useMemo(() => {
    if (!catalog || tokens.length > 0) return [];
    const counts = new Map<string, number>();
    for (const t of catalog) for (const tag of t.tags ?? []) counts.set(tag, (counts.get(tag) ?? 0) + 1);
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 12).map(([tag]) => tag);
  }, [catalog, tokens.length]);

  const goTo = useCallback(
    (path: string) => {
      onClose();
      navigate(path);
    },
    [navigate, onClose],
  );

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, Math.max(flatEntries.length - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const entry = flatEntries[active];
      if (entry) goTo(entry.path);
    }
  };

  // keep the active row visible
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${active}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [active]);

  const hasQuery = tokens.length > 0;
  const showAIList = aiMode && !aiBusy && !!aiHits;

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          key="search-overlay"
          className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] px-4"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          <div className="absolute inset-0 bg-black/40" onClick={onClose} />
          <motion.div
            className="relative w-full max-w-xl rounded-xl border border-bdr bg-bg-2 shadow-lg overflow-hidden"
            initial={{ opacity: 0, y: -8, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -8, scale: 0.98 }}
            transition={{ duration: 0.16 }}
          >
            {/* input row */}
            <div className="flex items-center gap-2 px-3.5 h-12 border-b border-bdr">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-tx-3 shrink-0">
                <circle cx="11" cy="11" r="7" />
                <line x1="21" y1="21" x2="16.5" y2="16.5" />
              </svg>
              <input
                ref={inputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder="Поиск задач: слова, теги или вопрос…"
                className="flex-1 bg-transparent outline-none text-sm text-tx-1 placeholder:text-tx-3"
              />
              {aiEnabled && (
                <button
                  type="button"
                  onClick={() => { if (!aiBusy) setAiMode((m) => !m); }}
                  className={clsx(
                    'shrink-0 inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-medium border transition-colors cursor-pointer',
                    aiMode ? 'bg-brand/15 text-brand border-brand/35' : 'bg-bg-3 text-tx-3 border-bdr-s hover:text-tx-2',
                  )}
                  title="Умный поиск (AI подбирает релевантные задачи по смыслу запроса)"
                >
                  ✦ Умный
                </button>
              )}
            </div>

            {/* results */}
            <div ref={listRef} className="max-h-[46vh] overflow-y-auto">
              {!hasQuery && (
                <div className="px-4 py-4">
                  <div className="text-xs text-tx-3 mb-2.5">Начните печатать… или выберите горячий тег:</div>
                  <div className="flex flex-wrap gap-1.5">
                    {hotTags.map((tag) => (
                      <TagBadge key={tag} tag={tag} size="sm" onClick={() => setQuery(tag)} />
                    ))}
                    {!hotTags.length && <span className="text-xs text-tx-3">Каталог задач ещё не загружен</span>}
                  </div>
                </div>
              )}

              {hasQuery && aiBusy && (
                <div className="px-4 py-6 text-sm text-tx-3 animate-pulse">AI подбирает задачи…</div>
              )}

              {hasQuery && !aiBusy && showAIList && (
                <div className="py-1.5">
                  {flatEntries.map((e, i) => (
                    <button
                      key={e.path}
                      type="button"
                      data-idx={i}
                      onMouseEnter={() => setActive(i)}
                      onClick={() => goTo(e.path)}
                      className={clsx(
                        'w-full text-left px-4 py-2.5 transition-colors cursor-pointer',
                        i === active ? 'bg-bg-4' : 'hover:bg-bg-3/60',
                      )}
                    >
                      <div className="text-sm text-tx-1 truncate">{e.title}</div>
                      <div className="text-xs text-tx-3 truncate mt-0.5">{e.subtitle}</div>
                      {e.reason && <div className="text-xs text-tx-2 truncate mt-0.5">{e.reason}</div>}
                    </button>
                  ))}
                  {flatEntries.length === 0 && (
                    <div className="px-4 py-4 text-sm text-tx-3">
                      Ничего не найдено по «{query.trim()}».
                      {results && results.length > 0 && ' Попробуйте обычный поиск.'}
                    </div>
                  )}
                </div>
              )}

              {hasQuery && !aiBusy && !showAIList && (
                <div className="py-1.5">
                  {loading && <div className="px-4 py-2 text-xs text-tx-3">Поиск…</div>}
                  {aiNotice && (
                    <div className="px-4 py-1.5 text-xs text-warn">{aiNotice}</div>
                  )}
                  {!loading && flatEntries.map((e, i) => {
                    const r = e.result!;
                    return (
                      <button
                        key={`${r.course}-${r.task}-${i}`}
                        type="button"
                        data-idx={i}
                        onMouseEnter={() => setActive(i)}
                        onClick={() => goTo(e.path)}
                        className={clsx(
                          'w-full text-left px-4 py-2.5 transition-colors cursor-pointer',
                          i === active ? 'bg-bg-4' : 'hover:bg-bg-3/60',
                        )}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="text-sm text-tx-1 truncate flex-1">
                            <Highlighted text={r.title} tokens={tokens} />
                          </span>
                          <DifficultyBadge difficulty={r.difficulty} size="sm" />
                          {r.rank === 'tag' && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-brand/10 text-brand border border-brand/25 shrink-0">тег</span>
                          )}
                          {r.rank === 'content' && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-bg-3 text-tx-3 border border-bdr-s shrink-0">условие</span>
                          )}
                        </div>
                        {r.snippet && (
                          <div className="text-xs text-tx-3 truncate mt-0.5">
                            <Highlighted text={r.snippet} tokens={tokens} />
                          </div>
                        )}
                        <div className="flex items-center gap-2 mt-1 flex-wrap min-w-0">
                          <span className="text-[11px] text-tx-3 shrink-0">{r.course} › {r.topic}</span>
                          {r.tags?.slice(0, 4).map((tag) => (
                            <TagBadge key={tag} tag={tag} size="sm" />
                          ))}
                        </div>
                      </button>
                    );
                  })}
                  {!loading && hasQuery && flatEntries.length === 0 && (
                    <div className="px-4 py-5 text-sm text-tx-3">
                      Ничего не найдено по «{query.trim()}»
                      {aiEnabled && !aiMode && (
                        <> — попробуйте <button type="button" onClick={() => setAiMode(true)} className="text-brand hover:underline cursor-pointer">умный поиск ✦</button></>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* footer hints */}
            <div className="flex items-center gap-3 px-4 h-8 border-t border-bdr text-[11px] text-tx-3">
              <span>↑↓ — навигация</span>
              <span>Enter — перейти</span>
              <span>Esc — закрыть</span>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
