import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import type { LangDriver, RunnerStatus } from '../api/types';
import { useTheme } from '../context/ThemeContext';
import { splitArgs, joinArgs } from '../lib/shlex';

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

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// Retry transient NotFoundError — dropped FileSystemEntry handles can momentarily
// fail to resolve (Chromium reclaims them under the hood); a quick retry succeeds.
async function withRetry<T>(fn: () => Promise<T>, attempts = 5): Promise<T> {
  let lastErr: any;
  for (let i = 0; i < attempts; i++) {
    try {
      return await fn();
    } catch (e: any) {
      lastErr = e;
      if (e?.name !== 'NotFoundError') throw e;
      await sleep(50 * (i + 1));
    }
  }
  throw lastErr;
}

// Promisified FileSystem API helpers — both success AND error callbacks, so a
// failed read REJECTS instead of hanging the promise forever (the old code passed
// no error callback, so any failure left the import silently stuck with no error).
function entryFile(entry: any): Promise<File> {
  return withRetry(() => new Promise<File>((resolve, reject) => entry.file(resolve, reject)));
}
function readEntriesBatch(reader: any): Promise<any[]> {
  return new Promise((resolve, reject) => reader.readEntries(resolve, reject));
}

// Recursively read a dropped FileSystemEntry into root-prefixed {file,path} pairs.
// Recursion is SEQUENTIAL on purpose: Chromium deadlocks (readEntries stops firing
// its callback) when too many FileSystemDirectoryReader instances run concurrently,
// which is what made deeper/larger course trees hang with no error or network call.
async function readEntry(entry: any, prefix: string): Promise<{ file: File; path: string }[]> {
  if (entry.isFile) {
    const f = await entryFile(entry);
    return [{ file: f, path: prefix + entry.name }];
  }
  const reader = entry.createReader();
  const children: any[] = [];
  // readEntries returns at most ~100 entries per call, so loop until it returns empty.
  for (;;) {
    const batch = await readEntriesBatch(reader);
    if (!batch.length) break;
    children.push(...batch);
  }
  const out: { file: File; path: string }[] = [];
  for (const child of children) {
    out.push(...(await readEntry(child, prefix + entry.name + '/')));
  }
  return out;
}

// Each dropped directory becomes one import source (a course, or a catalog folder).
// We START reading EAGERLY here, inside the drop handler, while the dropped
// FileSystemEntry handles are freshest — deferring the read (the old behaviour) let
// the handles go stale and the deep read failed with NotFoundError. The read promise
// is kicked off now; load() just awaits it, so the UI can still show pending jobs.
function sourcesFromDataTransfer(items: DataTransferItemList): ImportSource[] {
  return Array.from(items)
    .map((it) => it.webkitGetAsEntry?.())
    .filter((e: any) => e && e.isDirectory)
    .map((e: any) => {
      const filesPromise = readEntry(e, '');
      return { name: e.name, load: () => filesPromise };
    });
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

    // Process sources ONE AT A TIME. Reading a dropped directory tree spins up
    // FileSystemDirectoryReader instances; running several trees concurrently can
    // deadlock Chromium's reader, so we never read more than one tree at a time.
    for (let i = 0; i < sources.length; i++) {
      const id = start + i;
      try {
        await api.uploadCourseFiles(await sources[i].load());
        setJobs((js) => js.map((j) => (j.id === id ? { ...j, status: 'ok' } : j)));
      } catch (e) {
        setJobs((js) => js.map((j) => (j.id === id ? { ...j, status: 'error', error: (e as Error).message } : j)));
      }
    }
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
  {
    id: 'python3',
    name: 'Python',
    docsUrl: 'https://www.python.org/downloads/',
    install: [
      { os: 'Linux', cmd: 'sudo apt install python3 python3-pip\npip3 install pytest' },
      { os: 'macOS', cmd: 'brew install python\npip3 install pytest' },
      { os: 'Windows', cmd: 'winget install Python.Python.3.12\npip install pytest' },
    ],
  },
  {
    id: 'javascript',
    name: 'JavaScript (Node.js)',
    docsUrl: 'https://nodejs.org/',
    install: [
      { os: 'Linux', cmd: 'sudo apt install nodejs npm\nnpm i -g mocha' },
      { os: 'macOS', cmd: 'brew install node\nnpm i -g mocha' },
      { os: 'Windows', cmd: 'winget install OpenJS.NodeJS\nnpm i -g mocha' },
    ],
  },
  {
    id: 'cpp',
    name: 'C++',
    docsUrl: 'https://gcc.gnu.org/',
    install: [
      { os: 'Linux', cmd: 'sudo apt install g++ libgtest-dev' },
      { os: 'macOS', cmd: 'brew install gcc googletest' },
      { os: 'Windows', cmd: 'winget install GnuWin32.Make\n# MinGW-w64 + vcpkg install gtest' },
    ],
  },
  {
    id: 'java',
    name: 'Java',
    docsUrl: 'https://adoptium.net/',
    install: [
      {
        os: 'Linux',
        cmd: 'sudo apt install default-jdk\n'
          + '# JUnit 5: скачайте junit-platform-console-standalone.jar с Maven Central https://search.maven.org/artifact/org.junit.platform/junit-platform-console-standalone\n'
          + 'sudo mkdir -p /usr/share/java\n'
          + 'sudo mv ~/Downloads/junit-platform-console-standalone-*.jar /usr/share/java/junit-platform-console-standalone.jar',
      },
      { os: 'macOS', cmd: 'brew install openjdk\n# JUnit 5: скачайте junit-platform-console-standalone.jar с Maven Central https://search.maven.org/artifact/org.junit.platform/junit-platform-console-standalone\n# и сохраните как /usr/share/java/junit-platform-console-standalone.jar' },
      { os: 'Windows', cmd: 'winget install EclipseAdoptium.Temurin.21.JDK\n# JUnit 5: скачайте junit-platform-console-standalone.jar с Maven Central https://search.maven.org/artifact/org.junit.platform/junit-platform-console-standalone,\n# сохраните в удобную папку и укажите путь к ней в команде теста раннера (Settings → Раннеры → Java)' },
    ],
  },
  {
    id: 'csharp',
    name: 'C#',
    docsUrl: 'https://dotnet.microsoft.com/download',
    install: [
      { os: 'Linux', cmd: 'sudo apt install dotnet-sdk-10.0' },
      { os: 'macOS', cmd: 'brew install dotnet-sdk' },
      { os: 'Windows', cmd: 'winget install Microsoft.DotNet.SDK.10' },
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

function ResetIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 2v6h6" />
      <path d="M3 13a9 9 0 1 0 3-7.7L3 8" />
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
  broken:  { label: 'Тест не пройден',  color: 'text-warn',  Icon: AlertIcon },
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

function StatusBadge({ status, version, message, fetching }: { status?: RunnerStatus['status']; version?: string; message?: string; fetching: boolean }) {
  if (fetching) return <span className="flex items-center h-5 text-tx-3 text-xs whitespace-nowrap">Проверка…</span>;
  const sm = status ? STATUS_META[status] : null;
  if (!sm) return <span className="block h-5" />;
  return (
    <span
      className={clsx('flex items-center h-5 gap-1 text-xs whitespace-nowrap', sm.color, status === 'broken' && message && 'cursor-help')}
      title={status === 'broken' ? message : undefined}
    >
      <span className="inline-flex shrink-0 w-[13px] h-[13px]"><sm.Icon /></span>
      <span>{sm.label}{version ? ` (${version})` : ''}</span>
    </span>
  );
}

function RunnerCard({ def, driver, defaultDriver }: { def: RunnerDef; driver?: LangDriver; defaultDriver?: LangDriver }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<CardMode>('edit');
  const settled = useRef(false);

  // The saved driver loads/refreshes asynchronously; fall back to factory
  // defaults until it arrives so the fields are never blank.
  const baseRun = joinArgs(driver?.run_cmd ?? defaultDriver?.run_cmd ?? []);
  const baseTest = joinArgs(driver?.test_cmd ?? defaultDriver?.test_cmd ?? []);
  const defRun = joinArgs(defaultDriver?.run_cmd ?? []);
  const defTest = joinArgs(defaultDriver?.test_cmd ?? []);

  const [run, setRun] = useState(baseRun);
  const [test, setTest] = useState(baseTest);
  const [err, setErr] = useState('');

  // Re-sync fields when the saved/default commands change (initial load, save),
  // without clobbering in-progress edits (base only changes on load/save).
  const lastBase = useRef({ run: baseRun, test: baseTest });
  useEffect(() => {
    if (lastBase.current.run !== baseRun) { setRun(baseRun); lastBase.current.run = baseRun; }
    if (lastBase.current.test !== baseTest) { setTest(baseTest); lastBase.current.test = baseTest; }
  }, [baseRun, baseTest]);

  const canReset = !!defaultDriver && (run !== defRun || test !== defTest);
  const resetToDefaults = () => { setRun(defRun); setTest(defTest); };

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
      run_cmd: splitArgs(run),
      test_cmd: splitArgs(test),
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
  const message = detect.data?.message;
  const dirty = driver && (run !== joinArgs(driver.run_cmd) || test !== joinArgs(driver.test_cmd));

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
        <div className="min-w-0 flex-1">
          <p className="text-tx-1 text-sm font-medium break-words">{def.name}</p>
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
                  <StatusBadge status={status} version={version} message={message} fetching={detect.isFetching} />
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
                <StatusBadge status={status} version={version} message={message} fetching={detect.isFetching} />
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
                        <FormField label="Run" value={run} onChange={setRun} placeholder={defRun || 'команда запуска'} mono />
                        <FormField label="Test" value={test} onChange={setTest} placeholder={defTest || 'команда тестов'} mono />
                        {err && <p className="text-err text-xs">{err}</p>}
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => save.mutate()}
                            disabled={!dirty || save.isPending}
                            className="flex-1 h-8 rounded bg-brand text-white text-xs hover:opacity-90 disabled:opacity-40 transition-opacity"
                          >
                            {save.isPending ? 'Сохранение…' : 'Сохранить'}
                          </button>
                          <button
                            onClick={resetToDefaults}
                            disabled={!canReset}
                            title="Сбросить команды к значениям по умолчанию"
                            className="shrink-0 h-8 px-2 rounded border border-bdr text-tx-3 hover:text-tx-1 hover:bg-bg-4 disabled:opacity-40 disabled:cursor-default transition-colors"
                          >
                            <ResetIcon />
                          </button>
                        </div>
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

// maps the Go runtime.GOOS values reported by /detect to the `os` labels
// used in RunnerDef.install, so the prompt only ever includes commands for
// the host that is actually running the runners.
const PLATFORM_LABEL: Record<string, string> = {
  linux: 'Linux',
  darwin: 'macOS',
  windows: 'Windows',
};

function buildSetupPrompt(
  statuses: Record<string, RunnerStatus | undefined>,
  drivers: Record<string, LangDriver>,
): string {
  const platform = Object.values(statuses).find((s) => s?.platform)?.platform ?? 'unknown';
  const osLabel = PLATFORM_LABEL[platform] ?? platform;

  const lines: string[] = [];
  lines.push(
    'Ты — ассистент, который помогает настроить раннеры кода для CourseForge (self-hosted инструмент для запуска решений на разных языках).',
    'Настрой недостающие раннеры пошагово: давай ровно одну команду за раз и жди подтверждения, что она выполнена, прежде чем переходить к следующей.',
    '',
    `Платформа хоста: ${osLabel} (${platform})`,
    'Важно: команды должны быть универсальными для этой ОС (через системный пакетный менеджер), а НЕ завязанными на пути или версии конкретно этой машины — их можно будет повторить на другом устройстве с той же ОС.',
    '',
    'Текущее состояние раннеров:',
  );

  for (const def of RUNNERS) {
    const s = statuses[def.id];
    const driver = drivers[def.id];
    const statusLabel = s ? STATUS_META[s.status].label : 'неизвестно';
    const details = [statusLabel];
    if (s?.version) details.push(`версия ${s.version}`);
    if (s?.path) details.push(`путь: ${s.path}`);
    lines.push(`- ${def.name} (${def.id}): ${details.join(', ')}`);
    if (driver) {
      lines.push(`  текущая команда запуска: ${joinArgs(driver.run_cmd)}`);
      lines.push(`  текущая команда тестов: ${joinArgs(driver.test_cmd)}`);
    }
  }

  const needsSetup = RUNNERS.filter((def) => statuses[def.id]?.status !== 'ok');
  if (needsSetup.length > 0) {
    lines.push('', 'Раннеры, которые нужно настроить:');
    for (const def of needsSetup) {
      const s = statuses[def.id];
      lines.push('', `### ${def.name} (${def.id})`);
      if (s?.status === 'broken' && s.message) {
        lines.push(`Ошибка теста: ${s.message}`);
      }
      const install = def.install.find((it) => it.os === osLabel);
      lines.push(`Команды установки для ${osLabel}:`);
      lines.push(install ? install.cmd : '(нет готовой команды для этой ОС — предложи вариант через её обычный пакетный менеджер)');
      lines.push(`Документация: ${def.docsUrl}`);
    }
  } else {
    lines.push('', 'Все раннеры уже установлены и прошли проверку.');
  }

  lines.push(
    '',
    'Задача:',
    '1. Ответь одним сообщением: единый список команд для всех раннеров из «нужно настроить» подряд, в порядке установки, с кратким однострочным комментарием перед каждой командой (что она делает). Не разбивай ответ на несколько сообщений и не жди подтверждения между командами.',
    '2. Для каждой команды используй только универсальный пакетный менеджер указанной ОС — без путей, версий или шагов, специфичных для конкретной машины.',
    '3. В конце сообщения одним пунктом добавь: после выполнения команд нажать «Проверить» у каждой карточки в Settings → Раннеры, чтобы CourseForge переобнаружил раннеры.',
    '4. Если для раннера нет готовой команды — предложи вариант через стандартный пакетный менеджер той же ОС, не меняя платформу.',
  );

  return lines.join('\n');
}

function CopyIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function SetupPromptButton({ statuses, drivers }: { statuses: Record<string, RunnerStatus | undefined>; drivers: Record<string, LangDriver> }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(buildSetupPrompt(statuses, drivers));
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <button
      onClick={copy}
      title="Скопировать промпт для AI-агента с текущим статусом раннеров, чтобы настроить недостающие одним сообщением"
      className="flex items-center gap-1.5 text-tx-3 hover:text-tx-1 text-xs leading-4 transition-colors"
    >
      <span className="inline-flex items-center justify-center shrink-0 w-[13px] h-4">{copied ? <CheckIcon /> : <CopyIcon />}</span>
      {copied ? 'Скопировано' : 'Промпт для настройки'}
    </button>
  );
}

function RunnersSection() {
  const { data: runners = {} } = useQuery({
    queryKey: ['runners'],
    queryFn: api.listRunners,
  });
  const { data: defaults = {} } = useQuery({
    queryKey: ['runner-defaults'],
    queryFn: api.listRunnerDefaults,
    staleTime: Infinity,
  });
  // shares the ['runner-detect', id] cache with each RunnerCard's own query
  // (same key + staleTime: Infinity), so this doesn't trigger extra requests.
  const detectQueries = useQueries({
    queries: RUNNERS.map((def) => ({
      queryKey: ['runner-detect', def.id],
      queryFn: () => api.detectRunner(def.id),
      staleTime: Infinity,
      refetchOnWindowFocus: false,
    })),
  });
  const statuses = Object.fromEntries(RUNNERS.map((def, i) => [def.id, detectQueries[i].data]));

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <p className="text-tx-3 text-xs font-medium uppercase tracking-wide">Раннеры</p>
        <SetupPromptButton statuses={statuses} drivers={runners} />
      </div>
      <div className="space-y-2">
        {RUNNERS.map((def) => (
          <RunnerCard key={def.id} def={def} driver={runners[def.id]} defaultDriver={defaults[def.id]} />
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
