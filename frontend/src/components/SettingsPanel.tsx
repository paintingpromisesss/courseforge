import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import type { LangDriver, RunnerStatus } from '../api/types';
import { useTheme } from '../context/ThemeContext';

// ── helpers ───────────────────────────────────────────────────────────────────

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <p className="text-tx-3 text-xs font-medium uppercase tracking-wide mb-3">
      {children}
    </p>
  );
}

function FormField({
  label, value, onChange, placeholder, mono,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  mono?: boolean;
}) {
  return (
    <div>
      <label className="text-tx-3 text-xs block mb-1">{label}</label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={clsx(
          'w-full px-2 py-1.5 rounded bg-bg-3 border border-bdr text-tx-2 placeholder:text-tx-3 text-xs focus:outline-none focus:border-brand',
          mono && 'font-mono',
        )}
      />
    </div>
  );
}

// ── theme ─────────────────────────────────────────────────────────────────────

function ThemeSection() {
  const { theme, setTheme } = useTheme();
  return (
    <div>
      <SectionTitle>Тема</SectionTitle>
      <div className="flex gap-2">
        {(['dark', 'light'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTheme(t)}
            className={clsx(
              'flex-1 py-2 rounded text-sm transition-colors border',
              theme === t
                ? 'bg-brand border-brand text-white'
                : 'bg-bg-3 border-bdr text-tx-2 hover:text-tx-1 hover:bg-bg-4',
            )}
          >
            {t === 'dark' ? 'Тёмная' : 'Светлая'}
          </button>
        ))}
      </div>
    </div>
  );
}

// ── courses ───────────────────────────────────────────────────────────────────

type ImportJob = { id: number; name: string; status: 'pending' | 'ok' | 'error'; error?: string };
// name is known synchronously (for instant UI); files are read lazily on import.
type ImportSource = { name: string; load: () => Promise<{ file: File; path: string }[]> };

// Recursively read a dropped FileSystemEntry into root-prefixed {file,path} pairs.
function readEntry(entry: any, prefix: string): Promise<{ file: File; path: string }[]> {
  if (entry.isFile) {
    return new Promise((resolve) =>
      entry.file((f: File) => resolve([{ file: f, path: prefix + entry.name }])),
    );
  }
  const reader = entry.createReader();
  return new Promise((resolve) => {
    const collected: any[] = [];
    const readBatch = () =>
      // readEntries returns at most ~100 entries per call, so loop until empty
      reader.readEntries(async (batch: any[]) => {
        if (!batch.length) {
          const nested = await Promise.all(collected.map((e) => readEntry(e, prefix + entry.name + '/')));
          resolve(nested.flat());
          return;
        }
        collected.push(...batch);
        readBatch();
      });
    readBatch();
  });
}

// Each dropped directory becomes one import source (a course, or a catalog folder).
// Entries are captured synchronously (the DataTransfer is neutered after the handler);
// file contents are read lazily so the UI can show pending jobs immediately.
function sourcesFromDataTransfer(items: DataTransferItemList): ImportSource[] {
  return Array.from(items)
    .map((it) => it.webkitGetAsEntry?.())
    .filter((e: any) => e && e.isDirectory)
    .map((e: any) => ({ name: e.name, load: () => readEntry(e, '') }));
}

// A webkitdirectory picker yields files grouped under their first path segment.
function sourcesFromInput(list: FileList): ImportSource[] {
  const byRoot = new Map<string, { file: File; path: string }[]>();
  for (const file of Array.from(list)) {
    const path = file.webkitRelativePath || file.name;
    const root = path.split('/')[0];
    if (!byRoot.has(root)) byRoot.set(root, []);
    byRoot.get(root)!.push({ file, path });
  }
  return [...byRoot.entries()].map(([name, files]) => ({ name, load: async () => files }));
}

function CoursesSection() {
  const qc = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [jobs, setJobs] = useState<ImportJob[]>([]);
  const [dragOver, setDragOver] = useState(false);
  const idRef = useRef(0);

  const allDone = jobs.length > 0 && jobs.every((j) => j.status !== 'pending');

  const runImports = async (sources: ImportSource[]) => {
    if (!sources.length) return;
    const start = idRef.current;
    idRef.current = start + sources.length;
    // show pending jobs immediately (names are known sync); read files per-job after
    setJobs(sources.map((s, i) => ({ id: start + i, name: s.name, status: 'pending' })));

    await Promise.all(
      sources.map(async (src, i) => {
        const id = start + i;
        try {
          await api.uploadCourseFiles(await src.load());
          setJobs((js) => js.map((j) => (j.id === id ? { ...j, status: 'ok' } : j)));
        } catch (e) {
          setJobs((js) => js.map((j) => (j.id === id ? { ...j, status: 'error', error: (e as Error).message } : j)));
        }
      }),
    );
    // refresh catalogs first so newly imported catalog members are known before
    // the courses list updates — otherwise they flash as standalone courses
    await qc.refetchQueries({ queryKey: ['catalogs'] });
    qc.invalidateQueries({ queryKey: ['courses'] });
  };

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    runImports(sourcesFromDataTransfer(e.dataTransfer.items));
  };

  return (
    <div>
      <SectionTitle>Импорт Курсов</SectionTitle>

      <input
        ref={fileInputRef}
        type="file"
        className="hidden"
        // @ts-ignore
        webkitdirectory=""
        onChange={(e) => {
          if (e.target.files) runImports(sourcesFromInput(e.target.files));
          e.target.value = '';
        }}
      />
      <button
        onClick={() => fileInputRef.current?.click()}
        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
        onDragLeave={() => setDragOver(false)}
        onDrop={onDrop}
        className={clsx(
          'w-full flex flex-col items-center justify-center gap-1 px-4 py-7 rounded-lg border border-dashed text-center transition-colors',
          dragOver
            ? 'border-brand bg-brand/10 text-brand'
            : 'border-bdr bg-bg-3 text-tx-3 hover:border-brand/50 hover:text-tx-2',
        )}
      >
        <span className="text-xs">Перетащите курсы сюда</span>
        <span className="text-[11px] text-tx-3">или нажмите для выбора папки</span>
      </button>

      <AnimatePresence>
        {jobs.length > 0 && (
          <ImportModal jobs={jobs} done={allDone} onClose={() => setJobs([])} />
        )}
      </AnimatePresence>
    </div>
  );
}

function ImportModal({ jobs, done, onClose }: { jobs: ImportJob[]; done: boolean; onClose: () => void }) {
  return (
    <motion.div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 px-4"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
    >
      <motion.div
        className="w-full max-w-sm rounded-xl bg-bg-2 border border-bdr shadow-xl flex flex-col max-h-[80vh]"
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 8 }}
      >
        <h2 className="text-lg font-semibold text-tx-1 px-6 pt-6 pb-4">
          {done ? 'Импорт завершён' : 'Импорт курсов…'}
        </h2>
        <div className="flex-1 overflow-auto px-6 pb-4 space-y-2.5">
          {jobs.map((j) => (
            <div key={j.id} className="flex items-center gap-3">
              <span className="shrink-0 w-5 h-5 flex items-center justify-center">
                {j.status === 'pending' && (
                  <span className="text-tx-3 animate-spin inline-flex"><SpinnerIcon /></span>
                )}
                {j.status === 'ok' && <span className="text-ok"><CheckIcon /></span>}
                {j.status === 'error' && (
                  <span className="text-err cursor-help" title={j.error}><AlertIcon /></span>
                )}
              </span>
              <span className="text-tx-2 text-sm truncate">{j.name}</span>
            </div>
          ))}
        </div>
        <div className="flex justify-end px-6 py-4 border-t border-bdr">
          <button
            onClick={onClose}
            disabled={!done}
            className="px-4 h-9 rounded-lg bg-brand text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity"
          >
            Закрыть
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

function SpinnerIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
      <path d="M12 3a9 9 0 1 0 9 9" />
    </svg>
  );
}

// ── runners ───────────────────────────────────────────────────────────────────

interface RunnerDef {
  id: string;
  name: string;
  docsUrl: string;
  install: { os: string; cmd: string }[];
}

const RUNNERS: RunnerDef[] = [
  {
    id: 'go',
    name: 'Go',
    docsUrl: 'https://go.dev/dl/',
    install: [
      { os: 'Linux', cmd: 'sudo apt install golang-go\n# либо архив с go.dev/dl/ распаковать в /usr/local' },
      { os: 'macOS', cmd: 'brew install go' },
      { os: 'Windows', cmd: 'winget install GoLang.Go' },
    ],
  },
];

// icons
function PencilIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 20h9" />
      <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" />
    </svg>
  );
}

function BookIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
      <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Z" />
    </svg>
  );
}

function WrenchIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function AlertIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 9v4" />
      <path d="M12 17h.01" />
      <circle cx="12" cy="12" r="9" />
    </svg>
  );
}

const STATUS_META = {
  ok:      { label: 'Установлен',                  color: 'text-ok',         Icon: CheckIcon },
  broken:  { label: 'Установлен, тест не пройден',  color: 'text-warn',  Icon: AlertIcon },
  missing: { label: 'Не установлен',               color: 'text-tx-3',       Icon: AlertIcon },
} as const;

type CardMode = 'edit' | 'docs';

function Instructions({ def, status }: { def: RunnerDef; status?: RunnerStatus }) {
  return (
    <div className="space-y-2">
      {status?.status === 'broken' && status.message && (
        <pre className="text-err text-[11px] font-mono whitespace-pre-wrap break-words">{status.message}</pre>
      )}
      <p className="text-tx-3 text-xs">Установите {def.name} одним из способов:</p>
      {def.install.map((it) => (
        <div key={it.os}>
          <p className="text-tx-3 text-[11px] mb-0.5">{it.os}</p>
          <pre className="bg-bg-4 border border-bdr rounded px-2 py-1.5 text-tx-2 text-[11px] font-mono whitespace-pre-wrap break-words">{it.cmd}</pre>
        </div>
      ))}
      <a
        href={def.docsUrl}
        target="_blank"
        rel="noreferrer"
        className="inline-block text-brand text-xs hover:underline"
      >
        {def.docsUrl}
      </a>
      <p className="text-tx-3 text-[11px]">После установки нажмите «Проверить».</p>
    </div>
  );
}

function StatusBadge({ status, version, fetching }: { status?: RunnerStatus['status']; version?: string; fetching: boolean }) {
  if (fetching) return <span className="flex items-center h-5 text-tx-3 text-xs whitespace-nowrap">Проверка…</span>;
  const sm = status ? STATUS_META[status] : null;
  if (!sm) return <span className="block h-5" />;
  return (
    <span className={clsx('flex items-center h-5 gap-1 text-xs whitespace-nowrap', sm.color)}>
      <span className="inline-flex shrink-0 w-[13px] h-[13px]"><sm.Icon /></span>
      <span>{sm.label}{version ? ` (${version})` : ''}</span>
    </span>
  );
}

function RunnerCard({ def, driver }: { def: RunnerDef; driver?: LangDriver }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<CardMode>('edit');
  const settled = useRef(false);
  const [run, setRun] = useState((driver?.run_cmd ?? []).join(' '));
  const [test, setTest] = useState((driver?.test_cmd ?? []).join(' '));
  const [err, setErr] = useState('');

  const detect = useQuery({
    queryKey: ['runner-detect', def.id],
    queryFn: () => api.detectRunner(def.id),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

  // First time the status is known, expand to the instructions when the runner
  // needs attention; usable runners stay collapsed on the edit pane.
  useEffect(() => {
    if (!detect.data || settled.current) return;
    settled.current = true;
    if (detect.data.status !== 'ok') {
      setMode('docs');
      setOpen(true);
    }
  }, [detect.data?.status]);

  const save = useMutation({
    mutationFn: () => api.patchRunner(def.id, {
      run_cmd: run.trim().split(/\s+/),
      test_cmd: test.trim() ? test.trim().split(/\s+/) : [],
    }),
    onSuccess: () => {
      setErr('');
      qc.invalidateQueries({ queryKey: ['runners'] });
      detect.refetch();
    },
    onError: (e: Error) => setErr(e.message),
  });

  const status = detect.data?.status;
  const version = detect.data?.version;
  const dirty = driver && (run !== driver.run_cmd.join(' ') || test !== driver.test_cmd.join(' '));

  return (
    <div className="rounded bg-bg-3 border border-bdr overflow-hidden">
      {/* header — click to expand/collapse */}
      <button
        onClick={() => setOpen((v) => !v)}
        className={clsx(
          'w-full flex items-start justify-between gap-2 px-3 pt-2.5 text-left transition-[padding] duration-[220ms] ease-in-out',
          open ? 'pb-1.5' : 'pb-2.5',
        )}
      >
        <div className="min-w-0">
          <p className="text-tx-1 text-sm font-medium">{def.name}</p>
          {/* expanded: status sits under the name (shifted by text) */}
          <AnimatePresence initial={false}>
            {open && (
              <motion.div
                key="under"
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.22, ease: 'easeInOut' }}
                className="overflow-hidden"
              >
                <div className="mt-1.5">
                  <StatusBadge status={status} version={version} fetching={detect.isFetching} />
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* top-right slot: status when collapsed, edit/docs toggle when open */}
        <div className="shrink-0">
          <AnimatePresence mode="wait" initial={false}>
            {open ? (
              <motion.div
                key="toggle"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12 }}
                className="flex rounded border border-bdr overflow-hidden"
                onClick={(e) => e.stopPropagation()}
              >
                {([['edit', PencilIcon], ['docs', BookIcon]] as const).map(([m, Icon]) => (
                  <span
                    key={m}
                    role="button"
                    onClick={() => setMode(m)}
                    title={m === 'edit' ? 'Редактирование' : 'Инструкция'}
                    className={clsx(
                      'px-2 py-1 transition-colors cursor-pointer',
                      mode === m ? 'bg-brand text-white' : 'text-tx-3 hover:text-tx-1',
                    )}
                  >
                    <Icon />
                  </span>
                ))}
              </motion.div>
            ) : (
              <motion.div
                key="status"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12 }}
              >
                <StatusBadge status={status} version={version} fetching={detect.isFetching} />
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </button>

      {/* expandable body */}
      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            key="body"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.22, ease: 'easeInOut' }}
            className="overflow-hidden"
          >
            <div className="px-3 pb-3 space-y-3">
              {/* animated tab content */}
              <div className="relative">
                <AnimatePresence mode="wait" initial={false}>
                  <motion.div
                    key={mode}
                    initial={{ opacity: 0, x: mode === 'edit' ? -8 : 8 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: mode === 'edit' ? 8 : -8 }}
                    transition={{ duration: 0.15 }}
                  >
                    {mode === 'edit' ? (
                      <div className="space-y-2">
                        <FormField label="Run" value={run} onChange={setRun} placeholder="go run ." mono />
                        <FormField label="Test" value={test} onChange={setTest} placeholder="go test -v ." mono />
                        {err && <p className="text-err text-xs">{err}</p>}
                        <button
                          onClick={() => save.mutate()}
                          disabled={!dirty || save.isPending}
                          className="w-full h-8 rounded bg-brand text-white text-xs hover:opacity-90 disabled:opacity-40 transition-opacity"
                        >
                          {save.isPending ? 'Сохранение…' : 'Сохранить'}
                        </button>
                      </div>
                    ) : (
                      <Instructions def={def} status={detect.data} />
                    )}
                  </motion.div>
                </AnimatePresence>
              </div>

              {/* footer: detect / test trigger */}
              <div className="flex items-center gap-2 pt-3 border-t border-bdr">
                <button
                  onClick={() => detect.refetch()}
                  disabled={detect.isFetching}
                  title="Задетектить и протестировать раннер"
                  className="flex items-center gap-1.5 text-tx-3 hover:text-tx-1 text-xs leading-4 transition-colors disabled:opacity-50"
                >
                  <span className={clsx('inline-flex items-center justify-center shrink-0 w-[13px] h-4', detect.isFetching && 'animate-spin')}><WrenchIcon /></span>
                  {detect.isFetching ? 'Проверка…' : 'Проверить'}
                </button>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function RunnersSection() {
  const { data: runners = {} } = useQuery({
    queryKey: ['runners'],
    queryFn: api.listRunners,
  });

  return (
    <div>
      <SectionTitle>Раннеры</SectionTitle>
      <div className="space-y-2">
        {RUNNERS.map((def) => (
          <RunnerCard key={def.id} def={def} driver={runners[def.id]} />
        ))}
      </div>
    </div>
  );
}

// ── panel ─────────────────────────────────────────────────────────────────────

export function SettingsPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            className="fixed inset-0 z-40 bg-black/40"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
          />
          <motion.aside
            className="fixed right-0 top-0 bottom-0 z-50 w-80 bg-bg-2 border-l border-bdr flex flex-col"
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 300, damping: 30 }}
          >
            <div className="flex items-center justify-between px-4 h-11 border-b border-bdr shrink-0">
              <span className="text-tx-1 text-sm font-medium">Настройки</span>
              <button onClick={onClose} className="text-tx-3 hover:text-tx-1 text-lg leading-none">×</button>
            </div>
            <div className="overflow-y-auto flex-1 p-4 space-y-6">
              <ThemeSection />
              <div className="border-t border-bdr" />
              <CoursesSection />
              <div className="border-t border-bdr" />
              <RunnersSection />
            </div>
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  );
}
