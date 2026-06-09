import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import type { LangDriver } from '../api/types';
import { useTheme } from '../context/ThemeContext';
import { ProgressBar } from './ui/ProgressBar';

// ── templates ─────────────────────────────────────────────────────────────────

interface Template {
  id: string;
  name: string;
  defaultUrl: string;
  urlHint: string;
  pkg?: string;      // apt package name for systems without pre-built binary
  binPath: string;
  runCmd: string[];
  testCmd: string[];
  ext: string;
  testExt: string;
}

const TEMPLATES: Template[] = [
  {
    id: 'go', name: 'Go',
    defaultUrl: 'https://go.dev/dl/go1.22.4.linux-amd64.tar.gz',
    urlHint: 'https://go.dev/dl/',
    binPath: 'bin/go',
    runCmd: ['{bin}', 'run', '{file}'],
    testCmd: ['{bin}', 'test', '-v', '{file}'],
    ext: '.go', testExt: '_test.go',
  },
  {
    id: 'javascript', name: 'Node.js',
    defaultUrl: 'https://nodejs.org/dist/v20.13.1/node-v20.13.1-linux-x64.tar.gz',
    urlHint: 'https://nodejs.org/dist/',
    binPath: 'bin/node',
    runCmd: ['{bin}', '{file}'],
    testCmd: ['{bin}', '--test', '{testfile}'],
    ext: '.js', testExt: '.test.js',
  },
  {
    id: 'python', name: 'Python (PyPy)',
    defaultUrl: 'https://downloads.python.org/pypy/pypy3.10-v7.3.16-linux64.tar.bz2',
    urlHint: 'https://downloads.python.org/pypy/',
    binPath: 'bin/pypy3',
    runCmd: ['{bin}', '{file}'],
    testCmd: ['{bin}', '-m', 'pytest', '{testfile}', '-q'],
    ext: '.py', testExt: '_test.py',
  },
  {
    id: 'java', name: 'Java (Temurin 21)',
    defaultUrl: 'https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.3%2B9/OpenJDK21U-jdk_x64_linux_hotspot_21.0.3_9.tar.gz',
    urlHint: 'https://github.com/adoptium/temurin21-binaries/releases',
    binPath: 'bin/java',
    runCmd: ['{bin}', '{file}'],
    testCmd: [],
    ext: '.java', testExt: '',
  },
  {
    id: 'kotlin', name: 'Kotlin',
    defaultUrl: 'https://github.com/JetBrains/kotlin/releases/download/v1.9.23/kotlin-compiler-1.9.23.zip',
    urlHint: 'https://github.com/JetBrains/kotlin/releases',
    binPath: 'bin/kotlinc',
    runCmd: ['{bin}', '-script', '{file}'],
    testCmd: [],
    ext: '.kts', testExt: '',
  },
  {
    id: 'typescript', name: 'TypeScript (Deno)',
    defaultUrl: 'https://github.com/denoland/deno/releases/download/v1.43.6/deno-x86_64-unknown-linux-gnu.zip',
    urlHint: 'https://github.com/denoland/deno/releases',
    binPath: 'deno',
    runCmd: ['{bin}', 'run', '--allow-all', '{file}'],
    testCmd: ['{bin}', 'test', '--allow-all', '{file}'],
    ext: '.ts', testExt: '_test.ts',
  },
  {
    id: 'ruby', name: 'Ruby (TruffleRuby)',
    defaultUrl: 'https://github.com/oracle/truffleruby/releases/download/graal-24.0.1/truffleruby-24.0.1-linux-amd64.tar.gz',
    urlHint: 'https://github.com/oracle/truffleruby/releases',
    binPath: 'bin/ruby',
    runCmd: ['{bin}', '{file}'],
    testCmd: ['{bin}', '{testfile}'],
    ext: '.rb', testExt: '_test.rb',
  },
  {
    id: 'csharp', name: 'C# (.NET)',
    defaultUrl: 'https://dotnetcli.blob.core.windows.net/dotnet/Sdk/8.0.300/dotnet-sdk-8.0.300-linux-x64.tar.gz',
    urlHint: 'https://dotnet.microsoft.com/download',
    binPath: 'dotnet',
    runCmd: ['{bin}', 'csi', '{file}'],
    testCmd: [],
    ext: '.csx', testExt: '',
  },
  {
    id: 'rust', name: 'Rust',
    defaultUrl: '',
    urlHint: 'https://...',
    pkg: 'rustc',
    binPath: 'rustc',
    runCmd: ['sh', '-c', '{bin} {file} -o /tmp/cf_out_$$ && /tmp/cf_out_$$'],
    testCmd: [],
    ext: '.rs', testExt: '',
  },
  {
    id: 'cpp', name: 'C++',
    defaultUrl: '',
    urlHint: 'https://...',
    pkg: 'g++',
    binPath: 'g++',
    runCmd: ['sh', '-c', '{bin} {file} -o /tmp/cf_out_$$ && /tmp/cf_out_$$'],
    testCmd: [],
    ext: '.cpp', testExt: '',
  },
  {
    id: 'c', name: 'C',
    defaultUrl: '',
    urlHint: 'https://...',
    pkg: 'gcc',
    binPath: 'gcc',
    runCmd: ['sh', '-c', '{bin} {file} -o /tmp/cf_out_$$ && /tmp/cf_out_$$'],
    testCmd: [],
    ext: '.c', testExt: '',
  },
];

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

function CoursesSection() {
  const qc = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedFiles, setSelectedFiles] = useState<FileList | null>(null);
  const [serverPath, setServerPath] = useState('');
  const [uploadMsg, setUploadMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [importMsg, setImportMsg] = useState<{ ok: boolean; text: string } | null>(null);

  const uploadMut = useMutation({
    mutationFn: () => api.uploadCourse(selectedFiles!),
    onSuccess: (data) => {
      setUploadMsg({ ok: true, text: `Курс «${data.slug}» загружен` });
      setSelectedFiles(null);
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (e: Error) => setUploadMsg({ ok: false, text: e.message }),
  });

  const importMut = useMutation({
    mutationFn: () => api.importCourse(serverPath),
    onSuccess: (data) => {
      setImportMsg({ ok: true, text: `Курс «${data.slug}» импортирован` });
      setServerPath('');
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (e: Error) => setImportMsg({ ok: false, text: e.message }),
  });

  const folderName = selectedFiles?.[0]
    ? selectedFiles[0].webkitRelativePath.split('/')[0]
    : null;

  return (
    <div>
      <SectionTitle>Курсы</SectionTitle>

      <p className="text-tx-3 text-xs mb-2">С диска</p>
      <input
        ref={fileInputRef}
        type="file"
        className="hidden"
        // @ts-ignore
        webkitdirectory=""
        onChange={(e) => { setSelectedFiles(e.target.files); setUploadMsg(null); }}
      />
      <div className="flex gap-2 mb-1">
        <button
          onClick={() => fileInputRef.current?.click()}
          className="shrink-0 px-3 py-1.5 rounded bg-bg-4 border border-bdr text-tx-2 hover:text-tx-1 text-xs transition-colors"
        >
          Выбрать папку
        </button>
        {folderName && (
          <span className="flex-1 px-2 py-1.5 rounded bg-bg-3 border border-bdr text-tx-2 text-xs truncate">
            {folderName}
          </span>
        )}
        {selectedFiles && (
          <button
            onClick={() => uploadMut.mutate()}
            disabled={uploadMut.isPending}
            className="shrink-0 px-3 py-1.5 rounded bg-brand text-white text-xs hover:opacity-90 disabled:opacity-50 transition-opacity"
          >
            {uploadMut.isPending ? '...' : 'Загрузить'}
          </button>
        )}
      </div>
      {uploadMsg && (
        <p className={clsx('text-xs', uploadMsg.ok ? 'text-ok' : 'text-err')}>{uploadMsg.text}</p>
      )}

      <p className="text-tx-3 text-xs mt-4 mb-2">Путь на сервере</p>
      <div className="flex gap-2 mb-1">
        <input
          value={serverPath}
          onChange={(e) => { setServerPath(e.target.value); setImportMsg(null); }}
          placeholder="/path/to/course"
          className="flex-1 px-2 py-1.5 rounded bg-bg-3 border border-bdr text-tx-2 placeholder:text-tx-3 text-xs focus:outline-none focus:border-brand"
        />
        <button
          onClick={() => importMut.mutate()}
          disabled={!serverPath.trim() || importMut.isPending}
          className="shrink-0 px-3 py-1.5 rounded bg-brand text-white text-xs hover:opacity-90 disabled:opacity-50 transition-opacity"
        >
          {importMut.isPending ? '...' : 'Импорт'}
        </button>
      </div>
      {importMsg && (
        <p className={clsx('text-xs', importMsg.ok ? 'text-ok' : 'text-err')}>{importMsg.text}</p>
      )}
    </div>
  );
}

// ── install form ──────────────────────────────────────────────────────────────

interface InstallFormState {
  lang: string;
  pkg: string;
  url: string;
  binPath: string;
  run: string;
  test: string;
  ext: string;
  testExt: string;
}

function toFormState(t: Template): InstallFormState {
  return {
    lang: t.id,
    pkg: t.pkg ?? '',
    url: t.defaultUrl,
    binPath: t.binPath,
    run: t.runCmd.join(' '),
    test: t.testCmd.join(' '),
    ext: t.ext,
    testExt: t.testExt,
  };
}

function toEditState(t: Template, driver: LangDriver): InstallFormState {
  return {
    lang: t.id,
    pkg: t.pkg ?? '',
    url: '',
    binPath: t.binPath,
    run: driver.run_cmd.join(' '),
    test: driver.test_cmd.join(' '),
    ext: driver.ext,
    testExt: driver.test_ext,
  };
}

interface InstallFormProps {
  showLangField?: boolean;
  urlHint: string;
  initial: InstallFormState;
  installed?: boolean;
  onDone: () => void;
}

function TrashIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-1 14H6L5 6" />
      <path d="M10 11v6M14 11v6" />
      <path d="M9 6V4h6v2" />
    </svg>
  );
}

function InstallForm({ showLangField, urlHint, initial, installed, onDone }: InstallFormProps) {
  const qc = useQueryClient();
  const [form, setForm] = useState<InstallFormState>(initial);
  const [installing, setInstalling] = useState(false);
  const [error, setError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [installMode, setInstallMode] = useState<'apt' | 'url'>(initial.pkg && !initial.url ? 'apt' : 'url');

  useEffect(() => {
    if (!confirmDelete) return;
    const t = setTimeout(() => setConfirmDelete(false), 3000);
    return () => clearTimeout(t);
  }, [confirmDelete]);

  const field = (key: keyof InstallFormState) => (v: string) =>
    setForm((f) => ({ ...f, [key]: v }));

  const isPkgInstall = !installed && installMode === 'apt';
  const isPathInstall = !installed && installMode === 'url' && !form.url.trim();

  const { data: status } = useQuery({
    queryKey: ['install-status', form.lang],
    queryFn: () => api.getInstallStatus(form.lang),
    enabled: installing && !isPathInstall,
    refetchInterval: (q) => {
      const s = q.state.data?.status;
      return s === 'done' || s === 'error' ? false : 1500;
    },
  });

  useEffect(() => {
    if (!status) return;
    if (status.status === 'done') {
      qc.invalidateQueries({ queryKey: ['runners'] });
      setInstalling(false);
      onDone();
    } else if (status.status === 'error') {
      setError(status.message ?? 'Ошибка установки');
      setInstalling(false);
    }
  }, [status?.status]);

  const save = async () => {
    setError('');
    const lang = form.lang.trim();
    if (!lang) { setError('Укажите идентификатор языка'); return; }
    try {
      if (installed) {
        await api.addRunner(lang, {
          run_cmd: form.run.trim().split(/\s+/),
          test_cmd: form.test.trim() ? form.test.trim().split(/\s+/) : [],
          ext: form.ext.trim(),
          test_ext: form.testExt.trim(),
        });
        qc.invalidateQueries({ queryKey: ['runners'] });
        onDone();
      } else if (isPathInstall) {
        const resolve = (s: string) => s.replace(/\{bin\}/g, form.binPath.trim());
        await api.addRunner(lang, {
          run_cmd: form.run.trim().split(/\s+/).map(resolve),
          test_cmd: form.test.trim() ? form.test.trim().split(/\s+/).map(resolve) : [],
          ext: form.ext.trim(),
          test_ext: form.testExt.trim(),
        });
        qc.invalidateQueries({ queryKey: ['runners'] });
        onDone();
      } else if (isPkgInstall) {
        await api.installRunner({
          lang,
          pkg: form.pkg.trim(),
          bin_path: form.binPath.trim(),
          run_cmd: form.run.trim().split(/\s+/),
          test_cmd: form.test.trim() ? form.test.trim().split(/\s+/) : [],
          ext: form.ext.trim(),
          test_ext: form.testExt.trim(),
        });
        setInstalling(true);
      } else {
        await api.installRunner({
          lang,
          url: form.url.trim(),
          bin_path: form.binPath.trim(),
          run_cmd: form.run.trim().split(/\s+/),
          test_cmd: form.test.trim() ? form.test.trim().split(/\s+/) : [],
          ext: form.ext.trim(),
          test_ext: form.testExt.trim(),
        });
        setInstalling(true);
      }
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const handleDelete = async () => {
    if (!confirmDelete) { setConfirmDelete(true); return; }
    try {
      await api.deleteRunner(form.lang.trim());
      qc.invalidateQueries({ queryKey: ['runners'] });
      onDone();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const statusLabel =
    status?.status === 'downloading' ? `Загрузка ${status.progress}%`
    : status?.status === 'extracting' ? 'Распаковка...'
    : status?.status === 'installing' ? 'Установка пакета...'
    : installing ? 'Запуск...'
    : '';

  const isDirty = !installed || (
    form.run !== initial.run ||
    form.test !== initial.test ||
    form.ext !== initial.ext ||
    form.testExt !== initial.testExt
  );
  const canSave = Boolean(form.lang.trim() && form.binPath.trim() && form.run.trim() && form.ext.trim() && isDirty);
  const saveLabel = installing ? 'Установка...'
    : installed ? 'Изменить'
    : isPkgInstall ? 'Установить (apt)'
    : isPathInstall ? 'Добавить из PATH'
    : 'Установить';

  return (
    <div className="mt-2 p-3 rounded bg-bg-3 border border-bdr space-y-2">
      {!installed && (
        <div className="flex items-center justify-between">
          <span className="text-tx-3 text-xs">Установка</span>
          <div className="flex rounded border border-bdr overflow-hidden text-xs">
            {(['apt', 'url'] as const).map((m) => (
              <button
                key={m}
                onClick={() => setInstallMode(m)}
                className={clsx(
                  'px-2.5 py-0.5 transition-colors',
                  installMode === m ? 'bg-brand text-white' : 'text-tx-3 hover:text-tx-1',
                )}
              >
                {m === 'apt' ? 'apt' : 'URL'}
              </button>
            ))}
          </div>
        </div>
      )}
      {showLangField && (
        <FormField label="Идентификатор (lang)" value={form.lang} onChange={field('lang')} placeholder="python" mono />
      )}
      {!installed && isPkgInstall && (
        <FormField label="Пакет (apt)" value={form.pkg} onChange={field('pkg')} placeholder="gcc" mono />
      )}
      {!installed && !isPkgInstall && (
        <FormField
          label={isPathInstall ? 'URL архива (пусто = из PATH)' : 'Скачать (URL)'}
          value={form.url}
          onChange={field('url')}
          placeholder={urlHint}
          mono
        />
      )}
      {!installed && (
        <FormField label={isPkgInstall ? 'Имя бинаря' : 'Путь бинаря в архиве'} value={form.binPath} onChange={field('binPath')} placeholder="gcc" mono />
      )}
      <FormField label="Run" value={form.run} onChange={field('run')} placeholder="{bin} run {file}" mono />
      <FormField label="Test" value={form.test} onChange={field('test')} placeholder="{bin} test {file} {testfile}" mono />
      <div className="flex gap-2">
        <div className="flex-1">
          <FormField label="Расширение" value={form.ext} onChange={field('ext')} placeholder=".go" mono />
        </div>
        <div className="flex-1">
          <FormField label="Суффикс теста" value={form.testExt} onChange={field('testExt')} placeholder="_test.go" mono />
        </div>
      </div>

      {installing && status && <ProgressBar value={status.progress} max={100} />}
      {installing && statusLabel && <p className="text-tx-3 text-xs">{statusLabel}</p>}
      {error && <p className="text-err text-xs">{error}</p>}

      <div className="flex gap-2">
        <button
          onClick={save}
          disabled={!canSave || installing}
          className="flex-1 h-8 rounded bg-brand text-white text-xs hover:opacity-90 disabled:opacity-40 transition-opacity"
        >
          {saveLabel}
        </button>
        {installed && (
          <motion.button
            onClick={handleDelete}
            animate={{ width: confirmDelete ? 82 : 32 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
            className={clsx(
              'shrink-0 h-8 rounded border text-xs inline-flex items-center justify-center overflow-hidden transition-colors',
              confirmDelete
                ? 'bg-err/15 border-err text-err'
                : 'bg-bg-4 border-bdr text-tx-3 hover:text-err hover:border-err/50',
            )}
          >
            <AnimatePresence mode="wait" initial={false}>
              {confirmDelete ? (
                <motion.span
                  key="confirm"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.12 }}
                  className="whitespace-nowrap inline-flex items-center"
                >
                  Удалить?
                </motion.span>
              ) : (
                <motion.span
                  key="icon"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.12 }}
                  className="inline-flex items-center"
                >
                  <TrashIcon />
                </motion.span>
              )}
            </AnimatePresence>
          </motion.button>
        )}
      </div>
    </div>
  );
}

// ── runners ───────────────────────────────────────────────────────────────────

function RunnersSection() {
  const [openId, setOpenId] = useState<string | null>(null);
  const [customOpen, setCustomOpen] = useState(false);

  const { data: runners = {} } = useQuery({
    queryKey: ['runners'],
    queryFn: api.listRunners,
  });

  const installedLangs = new Set(Object.keys(runners));

  const sorted = [...TEMPLATES].sort((a, b) => {
    const ai = installedLangs.has(a.id);
    const bi = installedLangs.has(b.id);
    if (ai !== bi) return ai ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  const toggle = (id: string) => {
    setOpenId((prev) => (prev === id ? null : id));
    setCustomOpen(false);
  };

  const toggleCustom = () => {
    setCustomOpen((v) => !v);
    setOpenId(null);
  };

  return (
    <div>
      <SectionTitle>Раннеры</SectionTitle>

      <div className="space-y-1">
        {sorted.map((t) => {
          const installed = installedLangs.has(t.id);
          const isOpen = openId === t.id;
          return (
            <motion.div key={t.id} layout transition={{ duration: 0.25, ease: 'easeInOut' }}>
              <button
                onClick={() => toggle(t.id)}
                className="w-full flex items-center justify-between rounded bg-bg-3 border border-bdr px-3 py-2 transition-colors hover:border-bdr-e"
              >
                <span className={clsx('text-sm', installed ? 'text-tx-2' : 'text-tx-2')}>{t.name}</span>
                {installed
                  ? <span className={clsx('text-xs transition-colors', isOpen ? 'text-tx-3' : 'text-ok')}>{isOpen ? '−' : '✓'}</span>
                  : <span className="text-brand text-xs">{isOpen ? '−' : '+'}</span>
                }
              </button>
              <AnimatePresence>
                {isOpen && (
                  <motion.div
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: 'auto', opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={{ duration: 0.18 }}
                    className="overflow-hidden"
                  >
                    <InstallForm
                      urlHint={t.urlHint}
                      installed={installed}
                      initial={installed && runners[t.id] ? toEditState(t, runners[t.id]) : toFormState(t)}
                      onDone={() => setOpenId(null)}
                    />
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          );
        })}
      </div>

      <button
        onClick={toggleCustom}
        className="mt-4 w-full text-left text-tx-3 hover:text-tx-1 text-xs transition-colors flex items-center gap-1"
      >
        <span className="text-base leading-none">{customOpen ? '−' : '+'}</span>
        Свой раннер
      </button>
      <AnimatePresence>
        {customOpen && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.18 }}
            className="overflow-hidden"
          >
            <InstallForm
              showLangField
              urlHint="https://example.com/lang.tar.gz"
              initial={{ lang: '', pkg: '', url: '', binPath: '', run: '', test: '', ext: '', testExt: '' }}
              onDone={() => setCustomOpen(false)}
            />
          </motion.div>
        )}
      </AnimatePresence>
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
