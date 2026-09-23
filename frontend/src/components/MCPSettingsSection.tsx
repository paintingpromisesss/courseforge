import { useState, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { motion, AnimatePresence } from 'framer-motion';
import { api } from '../api/client';
import type { MCPConfig, MCPStatusResponse } from '../api/types';
import { buildMCPSetupPrompt } from '../lib/mcpPrompt';
import { buildMCPClients } from '../lib/mcpClients';

// ── Icons ──────────────────────────────────────────────────────────────────────

function CopyIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function BotIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="10" rx="2" />
      <circle cx="12" cy="5" r="2" />
      <path d="M12 7v4" />
      <line x1="8" y1="16" x2="8" y2="16" />
      <line x1="16" y1="16" x2="16" y2="16" />
    </svg>
  );
}

function WrenchIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

function TerminalMiniIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="4 17 10 11 4 5" />
      <line x1="12" y1="19" x2="20" y2="19" />
    </svg>
  );
}

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={clsx('transition-transform duration-200', open && 'rotate-180')}
    >
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

// ── MCP Tools Registry Metadata ────────────────────────────────────────────────

const MCP_TOOLS = [
  {
    name: 'list_courses',
    description: 'Возвращает полную иерархию каталогов, курсов, треков, тем и задач с их статусом прохождения.',
    category: 'Навигация',
  },
  {
    name: 'get_task_details',
    description: 'Получает условие задачи (markdown), поддерживаемые языки и ссылку на разбор решения.',
    category: 'Контент',
  },
  {
    name: 'get_task_template',
    description: 'Предоставляет стартовый код-шаблон для выбранной задачи на указанном языке.',
    category: 'Контент',
  },
  {
    name: 'get_task_solution',
    description: 'Возвращает авторское эталонное решение задачи (для проверки или подсказок).',
    category: 'Контент',
  },
  {
    name: 'get_task_tests',
    description: 'Возвращает тест-кейсы задачи для локальной валидации или анализа ошибок.',
    category: 'Тестирование',
  },
  {
    name: 'set_active_task_context',
    description: 'Устанавливает активную задачу в сессии CourseForge для быстрого доступа.',
    category: 'Контекст',
  },
  {
    name: 'get_current_task',
    description: 'Возвращает информацию о задаче, которая сейчас открыта пользователем в интерфейсе.',
    category: 'Контекст',
  },
  {
    name: 'run_solution',
    description: 'Запускает переданный код через штатный раннер CourseForge и возвращает результаты тестов.',
    category: 'Исполнение',
  },
  {
    name: 'list_submissions',
    description: 'Возвращает историю запусков и посылок по конкретной задаче с логами вывода.',
    category: 'Исполнение',
  },
];

// ── Main Component ────────────────────────────────────────────────────────────

export function MCPSettingsSection() {
  const { data: status, isLoading, isError } = useQuery({
    queryKey: ['mcp-config'],
    queryFn: api.mcpConfig,
    staleTime: 5000,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-16 text-tx-3 text-sm">
        Загрузка конфигурации MCP...
      </div>
    );
  }

  if (isError || !status) {
    return (
      <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
        Не удалось загрузить параметры MCP. Убедитесь, что бэкенд запущен.
      </div>
    );
  }

  return <MCPSettingsForm initialStatus={status} />;
}

function MCPSettingsForm({ initialStatus }: { initialStatus: MCPStatusResponse }) {
  const queryClient = useQueryClient();

  const [promptCopied, setPromptCopied] = useState(false);
  const [clientCopied, setClientCopied] = useState(false);
  const [clientId, setClientId] = useState('claude-code');
  const [cmdCopied, setCmdCopied] = useState(false);
  const [sseCopied, setSseCopied] = useState(false);
  const [showPromptPreview, setShowPromptPreview] = useState(false);
  const [showToolsList, setShowToolsList] = useState(false);

  // Local form state initialized from initialStatus
  const [enabled, setEnabled] = useState(initialStatus.enabled);
  const [transport, setTransport] = useState<'stdio' | 'sse'>(initialStatus.transport || 'stdio');
  const [host, setHost] = useState(initialStatus.host || '127.0.0.1');
  const [port, setPort] = useState(initialStatus.port || 8090);
  const [coursesDir, setCoursesDir] = useState(initialStatus.courses_dir || './courses');
  const [dataDir, setDataDir] = useState(initialStatus.data_dir || './data');
  const [isSaved, setIsSaved] = useState(false);

  const saveMutation = useMutation({
    mutationFn: (cfg: Partial<MCPConfig>) => api.mcpSaveConfig(cfg),
    onSuccess: (updated) => {
      queryClient.setQueryData(['mcp-config'], updated);
      setIsSaved(true);
      setTimeout(() => setIsSaved(false), 2000);
    },
  });

  const handleToggleEnabled = (val: boolean) => {
    setEnabled(val);
    saveMutation.mutate({ enabled: val });
  };

  const handleSaveForm = () => {
    saveMutation.mutate({
      enabled,
      transport,
      host,
      port: Number(port),
      courses_dir: coursesDir,
      data_dir: dataDir,
    });
  };

  const currentStatus: MCPStatusResponse = useMemo(() => {
    return {
      ...initialStatus,
      enabled,
      transport,
      host,
      port,
      courses_dir: coursesDir,
      data_dir: dataDir,
    };
  }, [initialStatus, enabled, transport, host, port, coursesDir, dataDir]);

  const generatedPrompt = useMemo(() => buildMCPSetupPrompt(currentStatus), [currentStatus]);

  const handleCopyPrompt = async () => {
    await navigator.clipboard.writeText(generatedPrompt);
    setPromptCopied(true);
    setTimeout(() => setPromptCopied(false), 2000);
  };

  const currentCommandStr = useMemo(() => {
    const command = currentStatus.command || currentStatus.binary_path || 'courseforge';
    const args = currentStatus.args && currentStatus.args.length > 0 ? currentStatus.args : ['mcp'];
    return `${command} ${args.join(' ')}`;
  }, [currentStatus]);

  const copyText = async (text: string, setCopiedState: (v: boolean) => void) => {
    await navigator.clipboard.writeText(text);
    setCopiedState(true);
    setTimeout(() => setCopiedState(false), 1500);
  };

  const clients = useMemo(() => buildMCPClients(currentStatus), [currentStatus]);
  const client = clients.find((c) => c.id === clientId) ?? clients[0];

  return (
    <div className="space-y-6 max-w-2xl">
      {/* ── 1. Header & Master Status Switch ─────────────────────────────── */}
      <div className="p-4 rounded-2xl bg-bg-2 border border-bdr flex items-center justify-between gap-4">
        <div className="flex items-center gap-3.5">
          <div
            className={clsx(
              'w-10 h-10 rounded-xl flex items-center justify-center transition-colors border',
              enabled
                ? 'bg-brand/10 border-brand/40 text-brand shadow-sm'
                : 'bg-bg-3 border-bdr text-tx-3',
            )}
          >
            <BotIcon />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-semibold text-tx-1">Model Context Protocol (MCP)</h3>
              <span
                className={clsx(
                  'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium tracking-wide uppercase',
                  enabled
                    ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                    : 'bg-amber-500/10 text-amber-400 border border-amber-500/30',
                )}
              >
                <span
                  className={clsx(
                    'w-1.5 h-1.5 rounded-full',
                    enabled ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400',
                  )}
                />
                {enabled ? 'Активен' : 'Отключен'}
              </span>
            </div>
            <p className="text-xs text-tx-3 mt-0.5">
              Интеграция CourseForge с внешними AI-ассистентами (Claude, Cursor, Antigravity)
            </p>
          </div>
        </div>

        {/* Master Toggle */}
        <label className="relative inline-flex items-center cursor-pointer select-none">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => handleToggleEnabled(e.target.checked)}
            className="sr-only peer"
          />
          <div className="w-11 h-6 bg-bg-3 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-brand" />
        </label>
      </div>

      {/* ── 2. Signature Action Card: Copy AI Agent Setup Prompt ────────── */}
      <div className="p-5 rounded-2xl bg-gradient-to-br from-brand/10 via-bg-2 to-bg-2 border border-brand/30 shadow-sm relative overflow-hidden group">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex-1 min-w-0 space-y-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold uppercase tracking-wider text-brand">
                Быстрая настройка агента
              </span>
              <span className="text-[11px] px-1.5 py-0.5 rounded bg-brand/20 text-brand font-medium">
                1 сообщение
              </span>
            </div>
            <h4 className="text-sm font-semibold text-tx-1">
              Промпт подключения для вашего AI-ассистента
            </h4>
            <p className="text-xs text-tx-3 max-w-md">
              Скопируйте сгенерированный промпт и отправьте его агенту. Он содержит готовые инструкции,
              пути к бинарнику и пошаговый тест подключения.
            </p>
          </div>

          <button
            type="button"
            onClick={handleCopyPrompt}
            className={clsx(
              'shrink-0 flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl font-medium text-xs whitespace-nowrap transition-colors shadow-sm cursor-pointer select-none',
              promptCopied
                ? 'bg-emerald-600 text-white shadow-emerald-900/30'
                : 'bg-brand hover:bg-brand/90 text-white shadow-brand/20',
            )}
          >
            <span className="shrink-0">
              {promptCopied ? <CheckIcon /> : <CopyIcon />}
            </span>
            <span className="grid">
              <span className={clsx('col-start-1 row-start-1 whitespace-nowrap', !promptCopied && 'invisible')}>
                Промпт скопирован!
              </span>
              <span className={clsx('col-start-1 row-start-1 whitespace-nowrap', promptCopied && 'invisible')}>
                Скопировать промпт
              </span>
            </span>
          </button>
        </div>

        {/* Prompt Preview Accordion */}
        <div className="mt-4 pt-3 border-t border-brand/20">
          <button
            type="button"
            onClick={() => setShowPromptPreview(!showPromptPreview)}
            className="flex items-center gap-1.5 text-xs text-tx-3 hover:text-tx-1 transition-colors cursor-pointer"
          >
            <ChevronIcon open={showPromptPreview} />
            <span>{showPromptPreview ? 'Скрыть текст промпта' : 'Показать сгенерированный промпт'}</span>
          </button>

          <AnimatePresence>
            {showPromptPreview && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                className="overflow-hidden"
              >
                <pre className="mt-2.5 p-3 rounded-xl bg-bg-3/90 border border-bdr text-[11px] font-mono text-tx-2 whitespace-pre-wrap leading-relaxed max-h-48 overflow-y-auto">
                  {generatedPrompt}
                </pre>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>

      {/* ── 3. Connection Configuration ─────────────────────────────────── */}
      <div className="p-5 rounded-2xl bg-bg-2 border border-bdr space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="text-sm font-semibold text-tx-1">Параметры соединения</h4>
            <p className="text-xs text-tx-3 mt-0.5">
              Транспорт и локальные пути для доступа к файлам курсов и базам решений
            </p>
          </div>
          {isSaved && (
            <span className="text-xs text-emerald-400 font-medium flex items-center gap-1">
              <CheckIcon /> Сохранено
            </span>
          )}
        </div>

        {/* Transport segmented switch */}
        <div>
          <label className="text-xs text-tx-3 block mb-1.5">Тип транспорта</label>
          <div className="grid grid-cols-2 gap-2 p-1 rounded-xl bg-bg-3 border border-bdr max-w-sm">
            <button
              type="button"
              onClick={() => setTransport('stdio')}
              className={clsx(
                'flex items-center justify-center gap-2 py-1.5 px-3 rounded-lg text-xs font-medium transition-all cursor-pointer',
                transport === 'stdio'
                  ? 'bg-bg-1 text-tx-1 shadow-sm border border-bdr'
                  : 'text-tx-3 hover:text-tx-2',
              )}
            >
              <TerminalMiniIcon />
              <span>stdio (рекомендуется)</span>
            </button>

            <button
              type="button"
              onClick={() => setTransport('sse')}
              className={clsx(
                'flex items-center justify-center gap-2 py-1.5 px-3 rounded-lg text-xs font-medium transition-all cursor-pointer',
                transport === 'sse'
                  ? 'bg-bg-1 text-tx-1 shadow-sm border border-bdr'
                  : 'text-tx-3 hover:text-tx-2',
              )}
            >
              <span>SSE (HTTP)</span>
            </button>
          </div>
          <p className="text-[11px] text-tx-3 mt-1.5">
            {transport === 'stdio'
              ? 'Агент запускает процесс courseforge напрямую через стандартный ввод/вывод.'
              : 'CourseForge поднимает постоянный HTTP-сервер со стримингом событий через Server-Sent Events.'}
          </p>
        </div>

        {/* SSE Host & Port when SSE selected */}
        {transport === 'sse' && (
          <div className="grid grid-cols-2 gap-3 pt-2">
            <div>
              <label className="text-tx-3 text-xs block mb-1">Хост</label>
              <input
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="127.0.0.1"
                className="w-full px-3 py-1.5 rounded-lg bg-bg-3 border border-bdr text-tx-2 text-xs font-mono focus:outline-none focus:border-brand"
              />
            </div>
            <div>
              <label className="text-tx-3 text-xs block mb-1">Порт</label>
              <input
                type="number"
                value={port}
                onChange={(e) => setPort(Number(e.target.value))}
                placeholder="8090"
                className="w-full px-3 py-1.5 rounded-lg bg-bg-3 border border-bdr text-tx-2 text-xs font-mono focus:outline-none focus:border-brand"
              />
            </div>
          </div>
        )}

        {/* Directories */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
          <div>
            <label className="text-tx-3 text-xs block mb-1">Директория курсов</label>
            <input
              value={coursesDir}
              onChange={(e) => setCoursesDir(e.target.value)}
              placeholder="./courses"
              className="w-full px-3 py-1.5 rounded-lg bg-bg-3 border border-bdr text-tx-2 text-xs font-mono focus:outline-none focus:border-brand"
            />
          </div>
          <div>
            <label className="text-tx-3 text-xs block mb-1">Директория данных</label>
            <input
              value={dataDir}
              onChange={(e) => setDataDir(e.target.value)}
              placeholder="./data"
              className="w-full px-3 py-1.5 rounded-lg bg-bg-3 border border-bdr text-tx-2 text-xs font-mono focus:outline-none focus:border-brand"
            />
          </div>
        </div>

        <div className="flex justify-end pt-2">
          <button
            type="button"
            onClick={handleSaveForm}
            disabled={saveMutation.isPending}
            className="px-4 py-1.5 rounded-xl bg-bg-3 hover:bg-bg-1 border border-bdr text-xs font-medium text-tx-1 transition-colors cursor-pointer"
          >
            {saveMutation.isPending ? 'Сохранение...' : 'Применить параметры'}
          </button>
        </div>
      </div>

      {/* ── 4. Launch Parameters for Agent / Client ─────────────────────── */}
      <div className="p-5 rounded-2xl bg-bg-2 border border-bdr space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="text-sm font-semibold text-tx-1">
              {transport === 'stdio' ? 'Параметры запуска (stdio)' : 'Параметры подключения (SSE HTTP)'}
            </h4>
            <p className="text-xs text-tx-3 mt-0.5">
              {transport === 'stdio'
                ? 'Агент запускает бинарник напрямую через стандартный ввод/вывод'
                : 'Агент подключается по сети к уже работающему серверу CourseForge'}
            </p>
          </div>
        </div>

        {transport === 'stdio' ? (
          <div className="space-y-2">
            {currentStatus.available === false && (
              <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 text-xs text-amber-300 flex items-start gap-2">
                <span className="shrink-0 mt-0.5 text-sm">⚠️</span>
                <div>
                  <span className="font-semibold block mb-0.5">Бинарник не найден на диске</span>
                  Бинарник CourseForge ещё не собран (или сервер запущен в режиме разработки через <code className="font-mono text-[11px] bg-bg-3 px-1 py-0.5 rounded text-tx-1">go run</code>).
                  Для работы по stdio соберите бинарник командой <code className="font-mono text-[11px] bg-bg-3 px-1 py-0.5 rounded text-tx-1">./scripts/build.sh</code> (на Windows: <code className="font-mono text-[11px] bg-bg-3 px-1 py-0.5 rounded text-tx-1">.\scripts\build.ps1</code>), либо используйте SSE-транспорт.
                </div>
              </div>
            )}
            <div className="p-2.5 rounded-xl bg-bg-3/60 border border-bdr flex items-center justify-between gap-2">
              <div className="min-w-0 flex-1">
                <span className="text-tx-3 text-[11px] block">Команда запуска (stdio):</span>
                <code className="text-tx-1 font-mono text-[11px] truncate block">{currentCommandStr}</code>
              </div>
              <button
                type="button"
                onClick={() => copyText(currentCommandStr, setCmdCopied)}
                className="shrink-0 flex items-center justify-center gap-1 px-3 py-1 rounded-lg bg-bg-2 hover:bg-bg-1 border border-bdr text-xs text-tx-2 hover:text-tx-1 transition-colors cursor-pointer select-none whitespace-nowrap"
              >
                <span className="grid">
                  <span className={clsx('col-start-1 row-start-1 whitespace-nowrap text-emerald-400 font-medium', !cmdCopied && 'invisible')}>
                    Скопировано
                  </span>
                  <span className={clsx('col-start-1 row-start-1 whitespace-nowrap', cmdCopied && 'invisible')}>
                    Копировать
                  </span>
                </span>
              </button>
            </div>
            <p className="text-[11px] text-tx-3 leading-relaxed px-1">
              Агент сам запускает процесс CourseForge в момент обращения. Веб-интерфейс CourseForge при этом может быть закрыт.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            <div className="p-2.5 rounded-xl bg-bg-3/60 border border-bdr flex items-center justify-between gap-2">
              <div className="min-w-0 flex-1">
                <span className="text-tx-3 text-[11px] block">SSE URL (HTTP):</span>
                <code className="text-tx-1 font-mono text-[11px] truncate block">
                  {currentStatus.sse_url || `http://${currentStatus.host || '127.0.0.1'}:${currentStatus.port || 8080}/api/mcp/sse`}
                </code>
              </div>
              <button
                type="button"
                onClick={() => copyText(currentStatus.sse_url || `http://${currentStatus.host || '127.0.0.1'}:${currentStatus.port || 8080}/api/mcp/sse`, setSseCopied)}
                className="shrink-0 flex items-center justify-center gap-1 px-3 py-1 rounded-lg bg-bg-2 hover:bg-bg-1 border border-bdr text-xs text-tx-2 hover:text-tx-1 transition-colors cursor-pointer select-none whitespace-nowrap"
              >
                <span className="grid">
                  <span className={clsx('col-start-1 row-start-1 whitespace-nowrap text-emerald-400 font-medium', !sseCopied && 'invisible')}>
                    Скопировано
                  </span>
                  <span className={clsx('col-start-1 row-start-1 whitespace-nowrap', sseCopied && 'invisible')}>
                    Копировать
                  </span>
                </span>
              </button>
            </div>
            <p className="text-[11px] text-tx-3 leading-relaxed px-1">
              Команда запуска не требуется: CourseForge уже запущен и обслуживает SSE-эндпоинт на текущем порту.
              (Для автономного запуска без GUI: <code className="px-1 py-0.5 rounded bg-bg-3 font-mono text-tx-2">courseforge mcp --transport=sse --port={port}</code>)
            </p>
          </div>
        )}

        <div className="pt-3 border-t border-bdr space-y-2.5">
          <div className="text-xs font-semibold text-tx-1">Подключение к агенту</div>
          <div className="flex flex-wrap gap-1.5">
            {clients.map((c) => (
              <button
                key={c.id}
                type="button"
                onClick={() => setClientId(c.id)}
                className={clsx(
                  'px-2.5 py-1 rounded-lg text-[11px] font-medium border transition-colors cursor-pointer',
                  c.id === client.id
                    ? 'bg-brand/10 border-brand/40 text-brand'
                    : 'bg-bg-3 border-bdr text-tx-3 hover:text-tx-1',
                )}
              >
                {c.name}
              </button>
            ))}
          </div>
          <p className="text-[11px] text-tx-3 leading-relaxed">
            Куда добавить: <code className="font-mono text-tx-2">{client.target}</code>
          </p>
          {client.snippet && (
            <div className="relative">
              <pre className="p-3 pr-28 rounded-xl bg-bg-3 border border-bdr text-xs font-mono text-tx-2 overflow-x-auto">
                {client.snippet}
              </pre>
              <button
                type="button"
                onClick={() => copyText(client.snippet, setClientCopied)}
                className="absolute top-2 right-2 flex items-center gap-1 px-2 py-1 rounded-lg bg-bg-2 hover:bg-bg-1 border border-bdr text-[11px] text-tx-2 hover:text-tx-1 transition-colors cursor-pointer select-none"
              >
                {clientCopied ? <CheckIcon /> : <CopyIcon />}
                <span>{clientCopied ? 'Скопировано' : 'Копировать'}</span>
              </button>
            </div>
          )}
          {client.note && <p className="text-[11px] text-tx-3 leading-relaxed">{client.note}</p>}
        </div>
      </div>

      {/* ── 5. Protocol Tools Registry ──────────────────────────────────── */}
      <div className="p-4 rounded-2xl bg-bg-2 border border-bdr space-y-3">
        <button
          type="button"
          onClick={() => setShowToolsList(!showToolsList)}
          className="w-full flex items-center justify-between text-left cursor-pointer group"
        >
          <div className="flex items-center gap-2.5">
            <div className="w-7 h-7 rounded-lg bg-bg-3 border border-bdr flex items-center justify-center text-tx-3 group-hover:text-brand transition-colors">
              <WrenchIcon />
            </div>
            <div>
              <div className="text-xs font-semibold text-tx-1">
                Доступные MCP-инструменты ({MCP_TOOLS.length})
              </div>
              <div className="text-[11px] text-tx-3">
                Функции, которые агент может вызывать в среде CourseForge
              </div>
            </div>
          </div>
          <ChevronIcon open={showToolsList} />
        </button>

        <AnimatePresence>
          {showToolsList && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="space-y-2 pt-2 border-t border-bdr overflow-hidden"
            >
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {MCP_TOOLS.map((tool) => (
                  <div
                    key={tool.name}
                    className="p-2.5 rounded-xl bg-bg-3/60 border border-bdr flex flex-col justify-between gap-1"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <code className="text-xs font-mono font-semibold text-brand">
                        {tool.name}
                      </code>
                      <span className="text-[10px] px-1.5 py-0.2 rounded bg-bg-2 border border-bdr text-tx-3">
                        {tool.category}
                      </span>
                    </div>
                    <p className="text-[11px] text-tx-3 leading-snug">{tool.description}</p>
                  </div>
                ))}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
