import { useState, useEffect, useLayoutEffect, useRef, useCallback, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';

import { api } from '../api/client';
import { Tabs } from '../components/ui/Tabs';
import { Markdown, VideoEmbed } from '../components/ui/Markdown';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { Badge } from '../components/ui/Badge';
import { loadCode, saveCode } from '../lib/editorStorage';
import { CodeMirrorEditor } from '../components/ui/CodeMirrorEditor';
import { LangSelect } from '../components/ui/LangSelect';
import { parseTestOutput, type ParsedResults } from '../lib/parseTests';
import { useTheme } from '../context/ThemeContext';
import type { Submission } from '../api/types';


function ResultsOverlay({
  results,
  durationMs,
  timedOut,
  collapsed,
  onToggleCollapse,
}: {
  results: ParsedResults;
  durationMs: number;
  timedOut: boolean;
  collapsed: boolean;
  onToggleCollapse: () => void;
}) {
  const sortedTests = [...results.tests].sort((a, b) => Number(b.passed) - Number(a.passed));
  const [selected, setSelected] = useState(sortedTests[0]?.name);
  const allPassed = results.passed === results.total;
  const pct = results.total > 0 ? Math.round((results.passed / results.total) * 100) : 0;

  const [testSidebarWidth, setTestSidebarWidth] = useState<number>(() => {
    const saved = localStorage.getItem('cf:test-sidebar-width');
    if (saved) {
      const parsed = parseInt(saved, 10);
      if (!isNaN(parsed) && parsed >= 160 && parsed <= 500) return parsed;
    }
    return 240;
  });

  const [isDragging, setIsDragging] = useState(false);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setIsDragging(true);
    const startX = e.clientX;
    const startWidth = testSidebarWidth;

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX;
      const newWidth = Math.min(Math.max(startWidth + delta, 160), 500);
      setTestSidebarWidth(newWidth);
      localStorage.setItem('cf:test-sidebar-width', String(newWidth));
    };

    const onMouseUp = () => {
      setIsDragging(false);
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }, [testSidebarWidth]);

  // sync selected when tests list changes (e.g. new submission)
  useEffect(() => {
    setSelected(sortedTests[0]?.name);
  }, [results]);

  /** Strip prefix up to last "/" and replace "_" with spaces */
  const displayName = (raw: string) => {
    const last = raw.lastIndexOf('/');
    return (last >= 0 ? raw.slice(last + 1) : raw).replaceAll('_', ' ');
  };

  return (
    <motion.div
      className="shrink-0 bg-bg-2 border-t border-bdr z-10"
      initial={{ height: 0, opacity: 0 }}
      animate={{ height: 'auto', opacity: 1 }}
      exit={{ height: 0, opacity: 0 }}
      transition={{ duration: 0.2 }}
    >
      {/* Header — clickable to collapse / expand */}
      <div
        onClick={onToggleCollapse}
        className={clsx(
          'flex items-center gap-3 px-4 py-2 cursor-pointer hover:bg-bg-3/50 select-none transition-colors',
          !collapsed && 'border-b border-bdr',
        )}
      >
        <span
          className={clsx(
            'inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold',
            allPassed ? 'bg-ok/15 text-ok' : 'bg-err/15 text-err',
          )}
        >
          {allPassed ? '✓ Принято' : '✗ Ошибка'}
        </span>
        <span className="text-tx-2 text-xs font-medium">
          {results.passed}/{results.total} тестов ({pct}%)
        </span>
        {timedOut && <Badge variant="warn">Timeout</Badge>}
        <span className="text-tx-3 text-xs">{durationMs}ms</span>

        {/* Collapse / expand toggle button */}
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleCollapse();
          }}
          title={collapsed ? 'Развернуть' : 'Свернуть'}
          className="ml-auto text-tx-3 hover:text-tx-1 p-1 rounded hover:bg-bg-4 transition-colors"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{
              transform: collapsed ? 'rotate(180deg)' : 'rotate(0deg)',
              transition: 'transform 0.2s',
            }}
          >
            <polyline points="4 10 8 6 12 10" />
          </svg>
        </button>
      </div>

      {/* Body — hidden when collapsed */}
      <AnimatePresence initial={false}>
        {!collapsed && (
          <motion.div
            key="body"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 230, opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.2 }}
            style={{ overflow: 'hidden' }}
          >
            <div className="flex overflow-hidden relative" style={{ height: 230 }}>
              {/* Left column: test list */}
              <div
                style={{ width: testSidebarWidth }}
                className={clsx(
                  'shrink-0 overflow-y-auto overflow-x-hidden',
                  isDragging && 'select-none',
                )}
              >
                {sortedTests.map((t) => (
                  <button
                    key={t.name}
                    onClick={() => setSelected(t.name)}
                    title={displayName(t.name)}
                    className={clsx(
                      'w-full flex items-center gap-2 px-3 py-1.5 text-left transition-colors text-xs',
                      selected === t.name ? 'bg-bg-4 text-tx-1' : 'text-tx-2 hover:bg-bg-3',
                    )}
                  >
                    <span
                      className={clsx(
                        'shrink-0 w-4 h-4 flex items-center justify-center rounded text-[10px] font-bold',
                        t.passed ? 'bg-ok/15 text-ok' : 'bg-err/15 text-err',
                      )}
                    >
                      {t.passed ? '✓' : '✗'}
                    </span>
                    <span className="truncate flex-1 font-medium">{displayName(t.name)}</span>
                  </button>
                ))}
              </div>

              {/* Standalone Splitter Handle */}
              <div
                onMouseDown={handleMouseDown}
                onDoubleClick={() => {
                  setTestSidebarWidth(240);
                  localStorage.setItem('cf:test-sidebar-width', '240');
                }}
                className={clsx(
                  'w-px shrink-0 bg-bdr cursor-col-resize z-20 transition-colors relative',
                  isDragging ? 'bg-brand shadow-[0_0_8px_rgba(124,58,237,0.5)]' : 'hover:bg-brand/60',
                )}
              >
                <div className="absolute inset-y-0 left-0 w-1.5 z-30" />
              </div>

              {/* Right column: test details */}
              <div className="flex-1 overflow-y-auto overflow-x-hidden p-3">
                {(() => {
                  const t = sortedTests.find((t) => t.name === selected);
                  if (!t) return null;
                  return t.detail ? (
                    <pre className="text-xs leading-5 text-tx-2 font-mono whitespace-pre-wrap break-words">
                      {t.detail}
                    </pre>
                  ) : (
                    <div className="flex items-center gap-2 text-xs text-tx-3">
                      <span className={t.passed ? 'text-ok' : 'text-err'}>
                        {t.passed ? '✓' : '✗'}
                      </span>
                      {t.passed ? 'Тест пройден, вывод отсутствует' : 'Нет данных об ошибке'}
                    </div>
                  );
                })()}
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}

function SubmissionDetail({
  submission,
  onLoadCode,
}: {
  submission: Submission;
  onLoadCode?: (code: string) => void;
}) {
  const [subTab, setSubTab] = useState<'code' | 'tests'>('code');
  const parsed = useMemo(
    () => parseTestOutput(submission.language, submission.stdout, submission.stderr, submission.exit_code),
    [submission]
  );
  const sortedTests = useMemo(
    () => [...parsed.tests].sort((a, b) => Number(b.passed) - Number(a.passed)),
    [parsed]
  );
  const [selectedTest, setSelectedTest] = useState<string>(sortedTests[0]?.name ?? '');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setSelectedTest(sortedTests[0]?.name ?? '');
  }, [sortedTests]);

  const displayName = (raw: string) => {
    const last = raw.lastIndexOf('/');
    return (last >= 0 ? raw.slice(last + 1) : raw).replaceAll('_', ' ');
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(submission.code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="bg-bg-1 border-t border-bdr-s">
      {/* Sub-tabs header */}
      <div className="flex items-center justify-between px-4 pt-1 border-b border-bdr-s bg-bg-2/40">
        <div className="flex gap-2">
          <button
            onClick={() => setSubTab('code')}
            className={clsx(
              'px-3 py-1.5 text-xs font-medium border-b-2 -mb-px transition-colors',
              subTab === 'code' ? 'border-brand text-tx-1' : 'border-transparent text-tx-3 hover:text-tx-2'
            )}
          >
            Код
          </button>
          <button
            onClick={() => setSubTab('tests')}
            className={clsx(
              'px-3 py-1.5 text-xs font-medium border-b-2 -mb-px transition-colors',
              subTab === 'tests' ? 'border-brand text-tx-1' : 'border-transparent text-tx-3 hover:text-tx-2'
            )}
          >
            Тесты ({parsed.passed}/{parsed.total})
          </button>
        </div>

        {subTab === 'code' && (
          <div className="flex items-center gap-1.5 pb-1">
            {onLoadCode && (
              <button
                onClick={() => onLoadCode(submission.code)}
                className="px-2 py-0.5 text-[11px] font-medium text-tx-3 hover:text-tx-1 hover:bg-bg-4 rounded transition-colors"
                title="Загрузить этот код в редактор"
              >
                Вставить в редактор
              </button>
            )}
            <button
              onClick={handleCopy}
              className="px-2 py-0.5 text-[11px] font-medium text-tx-3 hover:text-tx-1 hover:bg-bg-4 rounded transition-colors"
            >
              {copied ? 'Скопировано!' : 'Скопировать'}
            </button>
          </div>
        )}
      </div>

      {subTab === 'code' && (
        <div className="p-4 max-h-[650px] overflow-auto [&_pre]:my-0 [&_pre]:bg-transparent [&_pre]:border-0 [&_pre]:p-0">
          <Markdown content={`\`\`\`${submission.language}\n${submission.code}\n\`\`\``} />
        </div>
      )}

      {subTab === 'tests' && (
        <div className="flex flex-col sm:flex-row max-h-[650px]">
          {/* Test list */}
          <div className="sm:w-52 border-b sm:border-b-0 sm:border-r border-bdr-s overflow-y-auto py-1 shrink-0 bg-bg-2/20 max-h-[650px]">
            {sortedTests.map((t) => (
              <button
                key={t.name}
                onClick={() => setSelectedTest(t.name)}
                className={clsx(
                  'w-full flex items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors',
                  selectedTest === t.name ? 'bg-bg-4 text-tx-1 font-medium' : 'text-tx-2 hover:bg-bg-3'
                )}
              >
                <span className={clsx(
                  'shrink-0 w-4 h-4 flex items-center justify-center rounded text-[10px] font-bold',
                  t.passed ? 'bg-ok/15 text-ok' : 'bg-err/15 text-err'
                )}>
                  {t.passed ? '✓' : '✗'}
                </span>
                <span className="truncate">{displayName(t.name)}</span>
              </button>
            ))}
          </div>

          {/* Test output */}
          <div className="flex-1 overflow-auto p-3 bg-bg-1 max-h-[650px]">
            {(() => {
              const current = sortedTests.find((t) => t.name === selectedTest) ?? sortedTests[0];
              if (!current) return <div className="text-xs text-tx-3">Нет данных о тестах</div>;
              return current.detail ? (
                <pre className="text-xs leading-5 text-tx-2 font-mono whitespace-pre-wrap">
                  {current.detail}
                </pre>
              ) : (
                <div className="flex items-center gap-2 text-xs text-tx-3">
                  <span className={current.passed ? 'text-ok' : 'text-err'}>{current.passed ? '✓' : '✗'}</span>
                  {current.passed ? 'Тест пройден, вывод отсутствует' : 'Нет данных об ошибке'}
                </div>
              );
            })()}
          </div>
        </div>
      )}
    </div>
  );
}

function SolutionView({
  solution,
  lang,
  onLoadCode,
}: {
  solution: string;
  lang: string;
  onLoadCode?: (code: string) => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(solution);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div>
      {/* Pinned header toolbar */}
      <div className="sticky top-0 z-10 flex items-center justify-between px-4 py-2 border-b border-bdr bg-bg-2 shadow-sm text-xs">
        <div className="flex items-center gap-2">
          <span className="text-ok font-semibold">✓</span>
          <span className="font-semibold text-tx-1">Эталонное решение</span>
        </div>
        <div className="flex items-center gap-2">
          {onLoadCode && (
            <button
              onClick={() => onLoadCode(solution)}
              className="px-2.5 py-1 font-medium text-tx-2 hover:text-tx-1 hover:bg-bg-4 rounded transition-colors"
              title="Загрузить это решение в редактор"
            >
              Вставить в редактор
            </button>
          )}
          <button
            onClick={handleCopy}
            className="px-2.5 py-1 font-medium text-tx-2 hover:text-tx-1 hover:bg-bg-4 rounded transition-colors"
          >
            {copied ? 'Скопировано!' : 'Скопировать'}
          </button>
        </div>
      </div>

      {/* Code body */}
      <div className="p-4 [&_pre]:my-0 [&_pre]:bg-transparent [&_pre]:border-0 [&_pre]:p-0">
        <Markdown content={`\`\`\`${lang}\n${solution}\n\`\`\``} />
      </div>
    </div>
  );
}

function SubmissionsList({
  courseSlug,
  taskSlug,
  onLoadCode,
}: {
  courseSlug: string;
  taskSlug: string;
  onLoadCode?: (code: string) => void;
}) {
  const { data: subs, isLoading } = useQuery({
    queryKey: ['submissions', courseSlug, taskSlug],
    queryFn: () => api.listSubmissions(courseSlug, taskSlug),
  });
  const [expanded, setExpanded] = useState<number | null>(null);
  const [filter, setFilter] = useState<'all' | 'success' | 'failed'>('all');

  // Count occurrences of identical code among successful submissions
  const successCodeCounts = useMemo(() => {
    const counts = new Map<string, number>();
    if (!subs) return counts;
    for (const s of subs) {
      if (s.total_tests > 0 && s.passed_tests === s.total_tests) {
        const key = `${s.language}:::${s.code.trim()}`;
        counts.set(key, (counts.get(key) ?? 0) + 1);
      }
    }
    return counts;
  }, [subs]);

  if (isLoading) return <div className="p-4 text-tx-3 text-sm">Загрузка...</div>;
  if (!subs?.length) return <div className="p-4 text-tx-3 text-sm">Нет посылок</div>;

  const successCount = subs.filter((s) => s.total_tests > 0 && s.passed_tests === s.total_tests).length;
  const failedCount = subs.length - successCount;

  const filteredSubs = subs.filter((s) => {
    const isSuccess = s.total_tests > 0 && s.passed_tests === s.total_tests;
    if (filter === 'success') return isSuccess;
    if (filter === 'failed') return !isSuccess;
    return true;
  });

  return (
    <div>
      {/* Filter toolbar — pinned sticky at the top */}
      <div className="sticky top-0 z-10 flex items-center gap-1.5 p-2.5 border-b border-bdr bg-bg-2 shadow-sm text-xs">
        <button
          onClick={() => setFilter('all')}
          className={clsx(
            'px-2.5 py-1 rounded transition-colors font-medium',
            filter === 'all' ? 'bg-bg-4 text-tx-1 shadow-sm' : 'text-tx-3 hover:text-tx-2 hover:bg-bg-3'
          )}
        >
          Все ({subs.length})
        </button>
        <button
          onClick={() => setFilter('success')}
          className={clsx(
            'px-2.5 py-1 rounded transition-colors font-medium flex items-center gap-1',
            filter === 'success' ? 'bg-ok/15 text-ok font-semibold shadow-sm' : 'text-tx-3 hover:text-tx-2 hover:bg-bg-3'
          )}
        >
          <span>✓</span> Успешные ({successCount})
        </button>
        <button
          onClick={() => setFilter('failed')}
          className={clsx(
            'px-2.5 py-1 rounded transition-colors font-medium flex items-center gap-1',
            filter === 'failed' ? 'bg-err/15 text-err font-semibold shadow-sm' : 'text-tx-3 hover:text-tx-2 hover:bg-bg-3'
          )}
        >
          <span>✗</span> С ошибками ({failedCount})
        </button>
      </div>

      {filteredSubs.length === 0 ? (
        <div className="p-6 text-center text-tx-3 text-sm">
          {filter === 'success' ? 'Нет успешных посылок' : 'Нет посылок с ошибками'}
        </div>
      ) : (
        <div className="divide-y divide-bdr-s">
          {filteredSubs.map((s: Submission) => {
            const allPassed = s.total_tests > 0 && s.passed_tests === s.total_tests;
            const isDuplicate = allPassed && (successCodeCounts.get(`${s.language}:::${s.code.trim()}`) ?? 0) > 1;
            const date = new Date(s.created_at).toLocaleString('ru', {
              day: '2-digit',
              month: '2-digit',
              hour: '2-digit',
              minute: '2-digit',
            });

            return (
              <div key={s.id}>
                <button
                  onClick={() => setExpanded(expanded === s.id ? null : s.id)}
                  className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-bg-3 transition-colors text-left group"
                >
                  <span className={clsx('text-xs font-semibold w-16 shrink-0 flex items-center gap-1', allPassed ? 'text-ok' : 'text-err')}>
                    {allPassed ? '✓ Принято' : '✗ Ошибка'}
                  </span>
                  <span className="text-tx-2 text-xs font-medium shrink-0">{s.passed_tests}/{s.total_tests}</span>
                  <span className="text-tx-3 text-xs shrink-0">{s.duration_ms}ms</span>
                  <Badge variant="neutral" className="shrink-0">{s.language}</Badge>
                  {isDuplicate && (
                    <span
                      className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-bg-4 text-tx-3 border border-bdr shrink-0"
                      title="Есть другие успешные посылки с идентичным кодом"
                    >
                      Повтор кода
                    </span>
                  )}
                  <span className="ml-auto text-tx-3 text-xs shrink-0">{date}</span>
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 16 16"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className={clsx(
                      'text-tx-3 group-hover:text-tx-1 transition-transform shrink-0',
                      expanded === s.id ? 'rotate-180' : 'rotate-0'
                    )}
                  >
                    <polyline points="4 6 8 10 12 6" />
                  </svg>
                </button>

                <AnimatePresence initial={false}>
                  {expanded === s.id && (
                    <motion.div
                      key="detail"
                      initial={{ height: 0, opacity: 0 }}
                      animate={{ height: 'auto', opacity: 1 }}
                      exit={{ height: 0, opacity: 0 }}
                      transition={{ duration: 0.22, ease: 'easeInOut' }}
                      style={{ overflow: 'hidden' }}
                    >
                      <SubmissionDetail submission={s} onLoadCode={onLoadCode} />
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

type LeftTab = 'theory' | 'statement' | 'video' | 'submissions' | 'solution';

export function TaskPage() {
  const { courseSlug, trackSlug, topicSlug, unitSlug, taskSlug } = useParams<{
    courseSlug: string; trackSlug: string; topicSlug: string;
    unitSlug: string; taskSlug: string;
  }>();
  const qc = useQueryClient();
  const { theme } = useTheme();

  const { data: course } = useQuery({
    queryKey: ['course', courseSlug],
    queryFn: () => api.getCourse(courseSlug!),
    enabled: !!courseSlug,
  });

  const task = course?.tracks
    .flatMap((t) => t.topics)
    .flatMap((p) => p.units)
    .flatMap((u) => u.tasks)
    .find((t) => t.slug === taskSlug);

  const unit = course?.tracks
    .flatMap((t) => t.topics)
    .flatMap((p) => p.units)
    .find((u) => u.tasks.some((t) => t.slug === taskSlug));

  const [lang, setLang] = useState<string>('');
  const [code, setCode] = useState<string>('');
  const [leftTab, setLeftTab] = useState<LeftTab | null>(null);
  const initialTabSet = useRef(false);
  const prevTaskSlug = useRef<string | undefined>(undefined);
  const [showSolutionDialog, setShowSolutionDialog] = useState(false);
  // manual peek before solving; reset per task so the lock is unique per task
  const [solutionRevealed, setSolutionRevealed] = useState(false);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<{ parsed: ParsedResults; durationMs: number; timedOut: boolean } | null>(null);
  const [resultsCollapsed, setResultsCollapsed] = useState(false);
  const [markingTheoryDone, setMarkingTheoryDone] = useState(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const scrollPanelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (task?.languages?.length && !lang) {
      setLang(task.languages[0]);
    }
  }, [task, lang]);

  const { data: template } = useQuery({
    queryKey: ['template', courseSlug, trackSlug, topicSlug, unitSlug, taskSlug, lang],
    queryFn: () => api.getTemplate(courseSlug!, trackSlug!, topicSlug!, unitSlug!, taskSlug!, lang),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug && taskSlug && lang),
  });

  const { data: testCode } = useQuery({
    queryKey: ['tests', courseSlug, trackSlug, topicSlug, unitSlug, taskSlug, lang],
    queryFn: () => api.getTests(courseSlug!, trackSlug!, topicSlug!, unitSlug!, taskSlug!, lang),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug && taskSlug && lang),
  });

  useEffect(() => {
    if (!taskSlug || !lang || !template) return;
    const saved = loadCode(taskSlug, lang);
    setCode(saved ?? template);
  }, [taskSlug, lang, template]);

  const { data: theory } = useQuery({
    queryKey: ['theory', courseSlug, trackSlug, topicSlug, unitSlug],
    queryFn: () => api.getTheory(courseSlug!, trackSlug!, topicSlug!, unitSlug!),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug && unit?.has_theory),
  });

  const { data: statement } = useQuery({
    queryKey: ['statement', courseSlug, trackSlug, topicSlug, unitSlug, taskSlug],
    queryFn: () => api.getStatement(courseSlug!, trackSlug!, topicSlug!, unitSlug!, taskSlug!),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug && taskSlug),
  });

  const { data: submissions } = useQuery({
    queryKey: ['submissions', courseSlug, taskSlug],
    queryFn: () => api.listSubmissions(courseSlug!, taskSlug!),
    enabled: !!(courseSlug && taskSlug),
  });

  // lock auto-removes once this task has a successful submission
  const solved = !!submissions?.some((s) => s.total_tests > 0 && s.passed_tests === s.total_tests);
  const solutionUnlocked = solved || solutionRevealed;

  // Reset results and peek state when switching tasks
  useEffect(() => {
    setSolutionRevealed(false);
    setResults(null);
  }, [taskSlug]);

  // When submissions load for current task — load last submission result (collapsed by default)
  useEffect(() => {
    if (!submissions || submissions.length === 0) return;
    if (!results) {
      const last = submissions[0]; // API returns newest first
      const parsed = parseTestOutput(last.language, last.stdout, last.stderr, last.exit_code);
      setResults({ parsed, durationMs: last.duration_ms, timedOut: last.timed_out });
      setResultsCollapsed(true); // start collapsed when loaded from previous submissions
    }
  }, [taskSlug, submissions, results]);

  const { data: solution } = useQuery({
    queryKey: ['solution', courseSlug, trackSlug, topicSlug, unitSlug, taskSlug, lang],
    queryFn: () => api.getSolution(courseSlug!, trackSlug!, topicSlug!, unitSlug!, taskSlug!, lang),
    enabled: solutionUnlocked && !!(courseSlug && trackSlug && topicSlug && unitSlug && taskSlug && lang),
  });

  const { data: progress } = useQuery({
    queryKey: ['progress', courseSlug],
    queryFn: () => api.getProgress(courseSlug!),
    enabled: !!courseSlug,
  });

  // Runner status for the selected language (shared cache with LangSelect / Settings).
  const { data: runnerStatus } = useQuery({
    queryKey: ['runner-detect', lang],
    queryFn: () => api.detectRunner(lang),
    enabled: !!lang,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
  const runnerReady = runnerStatus?.status === 'ok';

  const theoryDone = !!(unitSlug && progress?.completed_tasks?.[unitSlug]);

  useEffect(() => {
    if (!unit || progress === undefined) return;
    if (prevTaskSlug.current !== taskSlug) {
      prevTaskSlug.current = taskSlug;
      initialTabSet.current = false;
    }
    if (initialTabSet.current) return;
    initialTabSet.current = true;
    setLeftTab(unit.has_theory && !theoryDone ? 'theory' : 'statement');
  }, [taskSlug, unit, progress, theoryDone]);

  const activeTab = leftTab ?? 'statement';

  const markTheoryDone = useCallback(async () => {
    if (!courseSlug || !unitSlug || theoryDone) return;
    setMarkingTheoryDone(true);
    try {
      await api.markDone(courseSlug, unitSlug, true);
      await qc.invalidateQueries({ queryKey: ['progress', courseSlug] });
      // course cards show done counts
      qc.invalidateQueries({ queryKey: ['courses'] });
      qc.invalidateQueries({ queryKey: ['catalogs'] });
    } finally {
      setMarkingTheoryDone(false);
    }
  }, [courseSlug, unitSlug, theoryDone, qc]);

  useLayoutEffect(() => {
    if (scrollPanelRef.current) scrollPanelRef.current.scrollTop = 0;
  }, [taskSlug]);

  const handleCodeChange = useCallback((val: string) => {
    setCode(val);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      if (taskSlug && lang) saveCode(taskSlug, lang, val);
    }, 1000);
  }, [taskSlug, lang]);

  const handleReset = async () => {
    if (!template) return;
    setCode(template);
    if (taskSlug && lang) saveCode(taskSlug, lang, template);
  };

  const handleSubmit = async () => {
    if (!lang || !code || !testCode) return;
    setRunning(true);
    setResults(null);
    try {
      // Run against the server's own copy of the tests (with task schema for
      // postgres) — this is both the authoritative score and the display data,
      // so results shown always match what got scored.
      const sub = await api.createSubmission({
        course_slug: courseSlug!,
        task_slug: taskSlug!,
        language: lang,
        code,
      });
      const parsed = parseTestOutput(lang, sub.stdout, sub.stderr, sub.exit_code);
      setResults({ parsed, durationMs: sub.duration_ms, timedOut: sub.timed_out });
      setResultsCollapsed(false); // expand panel after a fresh submit

      if (sub.total_tests > 0 && sub.passed_tests === sub.total_tests) {
        await api.markDone(courseSlug!, taskSlug!, true);
        qc.invalidateQueries({ queryKey: ['progress', courseSlug] });
        // course cards show done counts
        qc.invalidateQueries({ queryKey: ['courses'] });
        qc.invalidateQueries({ queryKey: ['catalogs'] });
      }

      qc.invalidateQueries({ queryKey: ['submissions', courseSlug, taskSlug] });
    } finally {
      setRunning(false);
    }
  };

  const leftTabs = [
    ...(unit?.has_theory ? [{ id: 'theory', label: 'Теория' }] : []),
    { id: 'statement', label: task?.title ?? 'Задача' },
    ...(task?.editorial_url ? [{ id: 'video', label: 'Видео-разбор' }] : []),
    { id: 'submissions', label: 'Посылки' },
    { id: 'solution', label: <span className="flex items-center gap-1">Решение {!solutionUnlocked && '🔒'}</span> },
  ] as { id: string; label: React.ReactNode }[];

  const handleTabChange = (id: string) => {
    if (id === 'solution' && !solutionUnlocked) {
      setShowSolutionDialog(true);
    } else {
      setLeftTab(id as LeftTab);
    }
  };

  const [leftPct, setLeftPct] = useState(45);
  const splitRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);

  const onDividerMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragging.current = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';

    const onMove = (ev: MouseEvent) => {
      if (!dragging.current || !splitRef.current) return;
      const rect = splitRef.current.getBoundingClientRect();
      const pct = ((ev.clientX - rect.left) / rect.width) * 100;
      setLeftPct(Math.min(80, Math.max(20, pct)));
    };
    const onUp = () => {
      dragging.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }, []);

  return (
    <div className="flex flex-col h-full">
      <div ref={splitRef} className="flex flex-1 overflow-hidden">
        <div style={{ width: `${leftPct}%` }} className="flex flex-col overflow-hidden shrink-0">
          <Tabs tabs={leftTabs} active={activeTab} onChange={handleTabChange} />
          <div ref={scrollPanelRef} className={clsx('flex-1 overflow-y-auto', (activeTab === 'submissions' || activeTab === 'solution') ? 'p-0' : 'p-4')}>
            {activeTab === 'theory' && (() => {
              const BASE = import.meta.env.VITE_API_URL ?? '/api';
              const assetBase = `${BASE}/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}`;
              return theory
                ? <>
                    {unit?.video_url && <VideoEmbed href={unit.video_url} />}
                    <Markdown content={theory} assetBase={assetBase} />
                    <div className="mt-8 flex justify-end">
                      <button
                        type="button"
                        onClick={() => void markTheoryDone().catch(() => {})}
                        disabled={theoryDone || markingTheoryDone}
                        className={clsx(
                          'rounded px-3 py-1.5 text-sm transition-colors',
                          theoryDone
                            ? 'bg-bg-4 text-ok'
                            : 'bg-brand text-white hover:bg-brand-hover disabled:opacity-70',
                        )}
                      >
                        {theoryDone ? 'Тема пройдена' : markingTheoryDone ? 'Сохраняю...' : 'Отметить тему пройденной'}
                      </button>
                    </div>
                  </>
                : <div className="text-tx-3 text-sm">Нет теории</div>;
            })()}
            {activeTab === 'statement' && (() => {
              const BASE = import.meta.env.VITE_API_URL ?? '/api';
              const assetBase = `${BASE}/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/tasks/${taskSlug}`;
              return statement
                ? <Markdown content={statement} assetBase={assetBase} />
                : <div className="text-tx-3 text-sm">Загрузка...</div>;
            })()}
            {activeTab === 'video' && task?.editorial_url && (
              <VideoEmbed href={task.editorial_url} />
            )}
            {activeTab === 'submissions' && (
              <SubmissionsList
                courseSlug={courseSlug!}
                taskSlug={taskSlug!}
                onLoadCode={(c) => {
                  setCode(c);
                  if (taskSlug && lang) saveCode(taskSlug, lang, c);
                }}
              />
            )}
            {activeTab === 'solution' && solutionUnlocked && (
              solution
                ? <SolutionView
                    solution={solution}
                    lang={lang}
                    onLoadCode={(c) => {
                      setCode(c);
                      if (taskSlug && lang) saveCode(taskSlug, lang, c);
                    }}
                  />
                : <div className="p-4 text-tx-3 text-sm">Загрузка...</div>
            )}
          </div>
        </div>

        <div
          onMouseDown={onDividerMouseDown}
          className="w-1 shrink-0 bg-bdr hover:bg-brand cursor-col-resize transition-colors"
        />

        <div className="flex-1 flex flex-col overflow-hidden relative">
          <div className="flex items-center gap-2 px-3 h-11 shrink-0 border-b border-bdr bg-bg-2">
            <LangSelect
              languages={task?.languages ?? []}
              value={lang}
              onChange={setLang}
            />
            <button
              onClick={handleReset}
              className="px-3 py-1 text-sm text-tx-2 hover:text-tx-1 hover:bg-bg-4 rounded transition-colors"
            >
              Сброс
            </button>
            {runnerStatus && !runnerReady && (
              <span className="ml-auto flex items-center gap-1.5 text-warn text-xs" title={runnerStatus.message}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
                  <line x1="12" y1="9" x2="12" y2="13" />
                  <line x1="12" y1="17" x2="12.01" y2="17" />
                </svg>
                Раннер {lang} не установлен
              </span>
            )}
            <button
              onClick={handleSubmit}
              disabled={running || !runnerReady}
              title={!runnerReady ? `Раннер для ${lang} недоступен` : undefined}
              className={clsx(
                'px-4 py-1.5 bg-brand hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm rounded transition-colors',
                runnerStatus && !runnerReady ? 'ml-2' : 'ml-auto',
              )}
            >
              {running ? 'Запуск...' : 'Отправить'}
            </button>
          </div>

          <div className="flex-1 overflow-hidden">
            <CodeMirrorEditor
              value={code}
              language={lang}
              isDark={theme === 'dark'}
              onChange={handleCodeChange}
            />
          </div>

          <AnimatePresence>
            {results && (
              <ResultsOverlay
                results={results.parsed}
                durationMs={results.durationMs}
                timedOut={results.timedOut}
                collapsed={resultsCollapsed}
                onToggleCollapse={() => setResultsCollapsed((v) => !v)}
              />
            )}
          </AnimatePresence>
        </div>
      </div>

      <ConfirmDialog
        open={showSolutionDialog}
        title="Показать эталонное решение?"
        message="Просмотр решения до самостоятельного решения задачи снижает его ценность."
        confirmLabel="Показать"
        onConfirm={() => {
          setSolutionRevealed(true);
          setLeftTab('solution');
          setShowSolutionDialog(false);
        }}
        onCancel={() => setShowSolutionDialog(false)}
      />
    </div>
  );
}
