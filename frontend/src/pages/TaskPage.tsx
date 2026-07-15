import { useState, useEffect, useLayoutEffect, useRef, useCallback } from 'react';
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
  onClose,
}: {
  results: ParsedResults;
  durationMs: number;
  timedOut: boolean;
  onClose: () => void;
}) {
  const sortedTests = [...results.tests].sort((a, b) => Number(b.passed) - Number(a.passed));
  const [selected, setSelected] = useState(sortedTests[0]?.name);
  const allPassed = results.passed === results.total;

  return (
    <motion.div
      className="absolute bottom-0 left-0 right-0 bg-bg-2 border-t border-bdr z-30"
      initial={{ y: '100%' }}
      animate={{ y: 0 }}
      exit={{ y: '100%' }}
      transition={{ type: 'spring', stiffness: 300, damping: 30 }}
    >
      <div className="flex items-center gap-3 px-4 py-2.5 border-b border-bdr">
        <span className={clsx('text-sm font-medium', allPassed ? 'text-ok' : 'text-err')}>
          {allPassed ? '✓ Принято' : '✗ Ошибка'}
        </span>
        <span className="text-tx-3 text-sm">{results.passed}/{results.total} тестов</span>
        {timedOut && <Badge variant="warn">Timeout</Badge>}
        <span className="text-tx-3 text-xs">{durationMs}ms</span>
        <button onClick={onClose} className="ml-auto text-tx-3 hover:text-tx-1 text-lg leading-none">×</button>
      </div>
      <div className="flex" style={{ height: 220 }}>
        <div className="w-48 border-r border-bdr overflow-y-auto py-1">
          {sortedTests.map((t) => (
            <button
              key={t.name}
              onClick={() => setSelected(t.name)}
              className={clsx(
                'w-full flex items-center gap-2 px-3 py-1.5 text-xs text-left transition-colors',
                selected === t.name ? 'bg-bg-4 text-tx-1' : 'text-tx-2 hover:bg-bg-3',
              )}
            >
              <span className={t.passed ? 'text-ok' : 'text-err'}>{t.passed ? '✓' : '✗'}</span>
              <span className="truncate">{t.name}</span>
            </button>
          ))}
        </div>
        <div className="flex-1 overflow-auto p-3">
          {(() => {
            const t = sortedTests.find((t) => t.name === selected);
            return t && (
              <pre className="text-xs text-tx-2 font-mono whitespace-pre-wrap">
                {t.detail || '(нет вывода)'}
              </pre>
            );
          })()}
        </div>
      </div>
    </motion.div>
  );
}

function SubmissionsList({ courseSlug, taskSlug }: { courseSlug: string; taskSlug: string }) {
  const { data: subs, isLoading } = useQuery({
    queryKey: ['submissions', courseSlug, taskSlug],
    queryFn: () => api.listSubmissions(courseSlug, taskSlug),
  });
  const [expanded, setExpanded] = useState<number | null>(null);

  if (isLoading) return <div className="p-4 text-tx-3 text-sm">Загрузка...</div>;
  if (!subs?.length) return <div className="p-4 text-tx-3 text-sm">Нет посылок</div>;

  return (
    <div className="divide-y divide-bdr-s">
      {subs.map((s: Submission) => {
        const allPassed = s.passed_tests === s.total_tests;
        const date = new Date(s.created_at).toLocaleString('ru', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
        return (
          <div key={s.id}>
            <button
              onClick={() => setExpanded(expanded === s.id ? null : s.id)}
              className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-bg-3 transition-colors text-left"
            >
              <span className={clsx('text-xs font-medium w-16 shrink-0', allPassed ? 'text-ok' : 'text-err')}>
                {allPassed ? 'Принято' : 'Ошибка'}
              </span>
              <span className="text-tx-3 text-xs">{s.passed_tests}/{s.total_tests}</span>
              <span className="text-tx-3 text-xs">{s.duration_ms}ms</span>
              <Badge variant="neutral" className="shrink-0">{s.language}</Badge>
              <span className="ml-auto text-tx-3 text-xs shrink-0">{date}</span>
            </button>
            {expanded === s.id && (
              <div className="bg-bg-1 border-t border-bdr-s px-4 py-3">
                <Markdown content={`<!-- code-snippets -->\n\`\`\`${s.language}\n${s.code}\n\`\`\``} />
              </div>
            )}
          </div>
        );
      })}
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

  // reset the manual peek when switching tasks
  useEffect(() => {
    setSolutionRevealed(false);
  }, [taskSlug]);

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
          <div ref={scrollPanelRef} className="flex-1 overflow-y-auto p-4">
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
              <SubmissionsList courseSlug={courseSlug!} taskSlug={taskSlug!} />
            )}
            {activeTab === 'solution' && solutionUnlocked && (
              solution
                ? <Markdown content={`<!-- code-snippets -->\n\`\`\`${lang}\n${solution}\n\`\`\``} />
                : <div className="text-tx-3 text-sm">Загрузка...</div>
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
                onClose={() => setResults(null)}
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
