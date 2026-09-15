import { useState, useRef, useEffect, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../../api/client';
import { AIChatMarkdown } from './AIChatMarkdown';
import { useResizable } from '../../hooks/useResizable';
import { useDraggable } from '../../hooks/useDraggable';
import type { AIConfig, AIMessage } from '../../api/types';

export type AIMode = 'docked' | 'floating' | 'closed';

const DEFAULT_AI_WIDTH = 440;
const MIN_AI_WIDTH = 360;
const MAX_AI_WIDTH = 800;

const FLOATING_WIDTH = 440;
const FLOATING_HEIGHT = 620;

const QUICK_PROMPTS = [
  '💡 Дай небольшую подсказку',
  '🔍 Объясни условие задачи простыми словами',
  '🛠 Какой алгоритмический подход здесь лучше использовать?',
];

interface ParsedContent {
  thinking: string | null;
  content: string;
  isLengthExceeded: boolean;
  isError: boolean;
  errorMessage: string | null;
  model: string | null;
  isReasoningMandatory: boolean;
}

function formatErrorMessage(raw: string): string {
  const jsonMatch = raw.match(/\{[\s\S]*\}/);
  if (jsonMatch) {
    try {
      const parsed = JSON.parse(jsonMatch[0]);
      if (parsed?.error) {
        const err = parsed.error;
        const msg = err.metadata?.raw || err.message || JSON.stringify(err);
        const code = err.code || err.status || '';
        return code ? `Ошибка ${code}: ${msg}` : msg;
      }
    } catch {
      // fallback to raw
    }
  }
  return raw;
}

function parseThinking(raw: string): ParsedContent {
  const isReasoningMandatory = raw.includes('<!-- CF_REASONING_MANDATORY -->');
  let model: string | null = null;
  const modelMatch = /<!-- CF_MODEL:(.*?) -->/.exec(raw);
  if (modelMatch) {
    model = modelMatch[1].trim();
  }

  if (raw.startsWith('<!-- CF_ERROR -->')) {
    const errorText = formatErrorMessage(raw.slice(17).trim());
    return {
      thinking: null,
      content: '',
      isLengthExceeded: false,
      isError: true,
      errorMessage: errorText,
      model,
      isReasoningMandatory,
    };
  }

  const isLengthExceeded = raw.includes('<!-- CF_FINISH_LENGTH -->');
  const clean = raw.replaceAll(/<!--\s*CF_[^>]*?(?:-->|$)/gs, '').trim();

  if (clean.includes('<think>')) {
    const thinkStartIndex = clean.indexOf('<think>');
    const thinkEndIndex = clean.indexOf('</think>');
    if (thinkEndIndex !== -1) {
      const thinking = clean.slice(thinkStartIndex + 7, thinkEndIndex).trim();
      const content = (clean.slice(0, thinkStartIndex) + clean.slice(thinkEndIndex + 8)).trim();
      return { thinking: thinking || null, content, isLengthExceeded, isError: false, errorMessage: null, model, isReasoningMandatory };
    } else {
      const thinking = clean.slice(thinkStartIndex + 7).trim();
      const content = clean.slice(0, thinkStartIndex).trim();
      return { thinking: thinking || null, content, isLengthExceeded, isError: false, errorMessage: null, model, isReasoningMandatory };
    }
  }

  if (/^(?:Here's a thinking process|Thinking process|Reasoning process):/i.test(clean)) {
    const markerMatch = /\n+(?:(?:All good[^\n]*|Ready to output[^\n]*|Final response[^\n]*|Output[^\n]*|Final Answer[^\n]*):?\s*(?:✅|💡|✨)?\s*|\n+(?=[А-ЯЁ][а-яё]+(?:!|\?|,|\.|\s)))/i.exec(clean);
    if (markerMatch) {
      const thinking = clean.slice(0, markerMatch.index).trim();
      const content = clean.slice(markerMatch.index + markerMatch[0].length).trim();
      return { thinking: thinking || null, content, isLengthExceeded, isError: false, errorMessage: null, model, isReasoningMandatory };
    }
    return { thinking: clean.trim(), content: '', isLengthExceeded, isError: false, errorMessage: null, model, isReasoningMandatory };
  }

  return { thinking: null, content: clean, isLengthExceeded, isError: false, errorMessage: null, model, isReasoningMandatory };
}

function ChatMessage({
  message,
  isStreaming,
}: {
  message: AIMessage;
  isStreaming?: boolean;
}) {
  const isUser = message.role === 'user';
  const { thinking, content, isLengthExceeded, isError, errorMessage, model, isReasoningMandatory } = isUser
    ? { thinking: null, content: message.content, isLengthExceeded: false, isError: false, errorMessage: null, model: null, isReasoningMandatory: false }
    : parseThinking(message.content);

  if (!isUser && isError) {
    return (
      <motion.div
        initial={{ opacity: 0, y: 6, scale: 0.99 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        transition={{ duration: 0.15 }}
        className="flex flex-col items-start w-full"
      >
        <div className="flex items-start gap-2 max-w-[95%] sm:max-w-[90%]">
          <div className="w-6 h-6 rounded-full bg-err/15 text-err flex items-center justify-center shrink-0 mt-0.5 border border-err/20">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
          </div>
          <div className="rounded-2xl rounded-tl-sm p-3 bg-err/10 border border-err/25 text-xs text-err flex flex-col gap-2 shadow-sm w-full">
            <div className="flex flex-col gap-1">
              <span className="font-semibold text-err flex items-center gap-1.5">
                <span>Ошибка обращения к модели</span>
              </span>
              <span className="text-tx-2 text-[11.5px] leading-relaxed break-words font-mono select-text">
                {errorMessage}
              </span>
            </div>
          </div>
        </div>
      </motion.div>
    );
  }

  if (!isUser && !content && !thinking) {
    return null;
  }

  const isContextExhausted = !isUser && !isStreaming && isLengthExceeded && (!content || !content.trim());

  return (
    <motion.div
      initial={{ opacity: 0, y: 6, scale: 0.99 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: 0.15 }}
      className={clsx('flex flex-col', isUser ? 'items-end' : 'items-start')}
    >
      <div className="flex items-end gap-2 max-w-[92%] sm:max-w-[88%]">
        {!isUser && (
          <div className="w-6 h-6 rounded-full bg-brand/15 text-brand flex items-center justify-center shrink-0 mb-1 border border-brand/20">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z" />
              <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z" />
            </svg>
          </div>
        )}
        <div
          className={clsx(
            'rounded-2xl px-3.5 py-2.5 text-sm break-words transition-colors',
            isUser
              ? 'bg-brand/20 text-tx-1 border border-brand/35 rounded-br-sm shadow-xs'
              : 'bg-bg-2 border border-bdr text-tx-1 rounded-bl-sm shadow-sm w-full'
          )}
        >
          {isUser ? (
            <p className="whitespace-pre-wrap leading-relaxed text-tx-1 text-[13.5px]">{message.content}</p>
          ) : (
            <div className="flex flex-col gap-2">
              {thinking && (
                <details className="rounded-xl bg-bg-3/60 border border-white/[0.08] overflow-hidden text-xs group">
                  <summary className="px-2.5 py-1.5 cursor-pointer select-none text-tx-3 hover:text-tx-2 font-medium flex items-center justify-between gap-1.5 bg-bg-3/40 hover:bg-bg-3 transition-colors list-none outline-none [&::-webkit-details-marker]:hidden">
                    <span className="inline-flex items-center gap-1.5">
                      {isStreaming && !content ? (
                        <span className="w-1.5 h-1.5 rounded-full bg-brand animate-pulse" />
                      ) : (
                        <svg
                          width="10"
                          height="10"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2.5"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          className="transition-transform group-open:rotate-90 text-tx-3"
                        >
                          <polyline points="9 18 15 12 9 6" />
                        </svg>
                      )}
                      <span>🧠 Ход мыслей</span>
                    </span>
                    <span className="text-[10px] text-tx-3/60 font-mono">
                      {thinking.length} симв.
                    </span>
                  </summary>
                  <div className="p-2.5 text-tx-3 text-[11px] leading-relaxed border-t border-white/[0.06] max-h-52 overflow-y-auto whitespace-pre-wrap font-mono bg-black/20 select-text">
                    {thinking}
                  </div>
                </details>
              )}

              {content ? (
                <AIChatMarkdown content={content} />
              ) : isStreaming && thinking ? (
                <div className="flex items-center gap-1.5 text-xs text-tx-3 italic py-0.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-brand/60 animate-bounce" />
                  <span>Формирование ответа...</span>
                </div>
              ) : null}

              {isContextExhausted && (
                <div className="p-3 bg-amber-500/10 border border-amber-500/30 rounded-xl text-xs text-amber-200 flex flex-col gap-2 my-0.5">
                  <div className="flex items-start gap-2">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-amber-400 mt-0.5">
                      <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" />
                      <line x1="12" y1="9" x2="12" y2="13" />
                      <line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <div className="flex flex-col gap-1">
                      <span className="font-semibold text-amber-300">Контекст модели слишком мал</span>
                      <span className="text-tx-2 text-[11.5px] leading-relaxed">
                        У модели недостаточно контекстного окна (или лимит токенов полностью исчерпан на этапе рассуждений). Ответ не сформирован.
                      </span>
                    </div>
                  </div>
                  <div className="pt-1 border-t border-amber-500/20">
                    <span className="text-[11px] text-tx-3">
                      Решение: выберите другую модель с большим контекстом или отключите режим мышления (🧠)
                    </span>
                  </div>
                </div>
              )}

              {isReasoningMandatory && (
                <div className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-brand/10 border border-brand/20 text-[11px] text-brand/90 select-none">
                  <span>🧠</span>
                  <span>Режим рассуждений включён по умолчанию для данной модели и не может быть отключён.</span>
                </div>
              )}

              {model && !isStreaming && (
                <div className="text-[11px] text-tx-3 italic mt-1 select-text truncate">
                  {model}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </motion.div>
  );
}



function HeaderModelDropdown({ config }: { config: AIConfig | null }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [isSwitching, setIsSwitching] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  useEffect(() => {
    if (open) {
      const timer = setTimeout(() => searchInputRef.current?.focus(), 50);
      return () => clearTimeout(timer);
    }
  }, [open]);

  const { data: models = [], isLoading, isError } = useQuery({
    queryKey: ['ai-models', config?.provider, config?.base_url],
    queryFn: () => {
      if (!config?.base_url) return [];
      return api.aiModels({
        provider: config.provider,
        base_url: config.base_url,
        api_key: config.api_key || '',
      });
    },
    enabled: open && !!config?.base_url,
    staleTime: 60 * 1000,
  });

  const handleSelectModel = async (modelId: string) => {
    if (!config || config.model === modelId || isSwitching) {
      setOpen(false);
      return;
    }
    setIsSwitching(true);
    try {
      await api.aiSaveConfig({
        provider: config.provider,
        base_url: config.base_url,
        api_key: config.api_key ? '********' : '',
        model: modelId,
        system_prompt: config.system_prompt,
      });
      await queryClient.invalidateQueries({ queryKey: ['ai-config'] });
      setOpen(false);
    } catch {
      // ignore
    } finally {
      setIsSwitching(false);
    }
  };

  if (!config?.enabled && !config?.model) {
    return (
      <span className="text-[10px] text-warn bg-warn/10 px-1.5 py-0.5 rounded border border-warn/20 font-medium shrink-0">
        Не настроен
      </span>
    );
  }

  const currentModel = config.model;
  const filteredModels = models.filter((m) =>
    m.id.toLowerCase().includes(search.toLowerCase().trim())
  );

  return (
    <div ref={rootRef} className="relative pointer-events-auto">
      <button
        type="button"
        onClick={() => {
          if (!open) setSearch('');
          setOpen((prev) => !prev);
        }}
        className={clsx(
          'flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-bg-3/90 hover:bg-bg-3 border text-xs font-mono transition-all cursor-pointer select-none max-w-[200px]',
          open
            ? 'border-brand ring-1 ring-brand/50 text-tx-1 bg-bg-3'
            : 'border-bdr hover:border-bdr-s text-tx-2 hover:text-tx-1'
        )}
        title={currentModel ? `Модель: ${currentModel}` : 'Выбрать модель'}
      >
        <span className="w-1.5 h-1.5 rounded-full bg-brand shrink-0" />
        <span className="truncate font-medium text-[11px]">{currentModel || 'Выбрать модель'}</span>
        <svg
          width="10"
          height="10"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={clsx('text-tx-3 shrink-0 ml-0.5 transition-transform duration-150', open && 'rotate-180')}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1.5 z-50 w-72 max-w-[calc(100vw-32px)] rounded-xl border border-bdr bg-bg-1 shadow-2xl overflow-hidden flex flex-col">
          {/* Search bar */}
          <div className="p-2 pb-1 shrink-0">
            <div className="relative flex items-center">
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="absolute left-2.5 text-tx-3 pointer-events-none"
              >
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
              <input
                ref={searchInputRef}
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Поиск модели..."
                className="w-full bg-bg-3 border border-bdr rounded-lg pl-7 pr-6 py-1 text-xs text-tx-1 placeholder:text-tx-3/60 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono transition-colors"
              />
              {search && (
                <button
                  type="button"
                  onClick={() => setSearch('')}
                  className="absolute right-2 text-tx-3 hover:text-tx-1 p-0.5 cursor-pointer text-xs leading-none"
                >
                  ✕
                </button>
              )}
            </div>
          </div>

          {/* Scrollable models list */}
          <div className="max-h-64 overflow-y-auto p-1 space-y-0.5 select-none">
            {isLoading ? (
              <div className="py-6 text-center text-xs text-tx-3 flex items-center justify-center gap-2">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="animate-spin text-brand">
                  <path d="M21 12a9 9 0 1 1-6.219-8.56" />
                </svg>
                <span>Загрузка моделей...</span>
              </div>
            ) : isError ? (
              <div className="py-4 px-3 text-center text-xs text-err">
                Не удалось загрузить список моделей
              </div>
            ) : filteredModels.length === 0 ? (
              <div className="py-4 text-center text-xs text-tx-3">
                {search ? 'Модели не найдены' : 'Список моделей пуст'}
              </div>
            ) : (
              filteredModels.map((m) => {
                const isSelected = m.id === currentModel;
                return (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => handleSelectModel(m.id)}
                    className={clsx(
                      'w-full flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg text-left text-xs font-mono transition-all cursor-pointer',
                      isSelected
                        ? 'bg-brand/15 text-brand font-semibold'
                        : 'text-tx-2 hover:text-tx-1 hover:bg-bg-3'
                    )}
                  >
                    <div className="flex items-center gap-1.5 min-w-0 flex-1">
                      <span className="truncate">{m.id}</span>
                      {!m.available && (
                        <span className="text-[10px] px-1.5 py-0.2 rounded bg-err/10 text-err/80 border border-err/20 shrink-0 font-sans">
                          недоступна
                        </span>
                      )}
                    </div>
                    {isSelected && (
                      <svg
                        width="13"
                        height="13"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        className="shrink-0 text-brand ml-1"
                      >
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                    )}
                  </button>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function AIChatPanel({
  mode,
  onModeChange,
  messages,
  onClearChat,
  aiConfig,
  input,
  onInputChange,
  onSend,
  onStop,
  streaming,
  streamingContent,
  dragHandleProps,
}: {
  mode: 'docked' | 'floating';
  onModeChange: (nextMode: AIMode) => void;
  messages: AIMessage[];
  onClearChat: () => void;
  aiConfig: AIConfig | null;
  input: string;
  onInputChange: (val: string) => void;
  onSend: (text?: string) => void;
  onStop: () => void;
  streaming: boolean;
  streamingContent: string;
  dragHandleProps?: {
    onPointerDown?: (e: React.PointerEvent) => void;
    style?: React.CSSProperties;
  };
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const isAtBottomRef = useRef(true);

  const handleScroll = () => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    isAtBottomRef.current = scrollHeight - scrollTop - clientHeight < 60;
  };

  useEffect(() => {
    if (scrollRef.current && isAtBottomRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, streaming, streamingContent]);

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden select-text">
      {/* Header bar */}
      <div
        {...(mode === 'floating' ? dragHandleProps : {})}
        className={clsx(
          'flex items-center justify-between px-3 h-11 border-b border-bdr bg-bg-2 shrink-0 select-none',
          mode === 'floating' && 'cursor-grab active:cursor-grabbing'
        )}
      >
        <div className="flex-1 min-w-0 flex items-center overflow-visible">
          <div className="flex items-center gap-2 min-w-0">
            <div className="w-6 h-6 rounded-lg bg-brand/15 text-brand flex items-center justify-center border border-brand/20 shadow-xs shrink-0">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z" />
                <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z" />
              </svg>
            </div>
            <div className="flex items-center gap-2 min-w-0">
              <span className="text-xs font-semibold text-tx-1 shrink-0">AI</span>
              <HeaderModelDropdown config={aiConfig} />
            </div>
          </div>
        </div>

        {/* Header Action Controls */}
        <div className="flex items-center gap-1 pointer-events-auto">
          {/* Mode Switcher */}
          {mode === 'docked' ? (
            <button
              type="button"
              onClick={() => onModeChange('floating')}
              className="w-7 h-7 rounded-lg flex items-center justify-center text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
              title="Открепить в плавающее окно"
            >
              {/* Pop-out icon */}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M15 3h6v6" />
                <path d="M10 14 21 3" />
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
              </svg>
            </button>
          ) : (
            <button
              type="button"
              onClick={() => onModeChange('docked')}
              className="w-7 h-7 rounded-lg flex items-center justify-center text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
              title="Закрепить на боковой панели"
            >
              {/* Dock icon */}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <rect width="18" height="18" x="3" y="3" rx="2" />
                <path d="M15 3v18" />
                <path d="m8 9 3 3-3 3" />
              </svg>
            </button>
          )}

          {/* Clear chat */}
          {messages.length > 0 && (
            <button
              type="button"
              onClick={onClearChat}
              className="w-7 h-7 rounded-lg flex items-center justify-center text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
              title="Очистить чат"
            >
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
              </svg>
            </button>
          )}

          {/* Minimize / Close */}
          <button
            type="button"
            onClick={() => onModeChange('closed')}
            className="w-7 h-7 rounded-lg flex items-center justify-center text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
            title="Закрыть"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
              <path d="M18 6 6 18M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      {/* Main content: chat view */}
      <div className="flex-1 min-h-0 overflow-hidden relative bg-bg-1 flex flex-col">
        {/* Scrollable messages list */}
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto p-3.5 space-y-3"
        >
                {messages.length === 0 && !streaming && (
                  <div className="h-full flex flex-col items-center justify-center text-center py-6 px-3">
                    <div className="w-11 h-11 rounded-2xl bg-brand/10 border border-brand/20 text-brand flex items-center justify-center mb-2.5 shadow-inner">
                      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z" />
                        <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z" />
                      </svg>
                    </div>
                    <h3 className="text-xs sm:text-sm font-semibold text-tx-1 mb-1">Чем помочь с задачей?</h3>
                    <p className="text-[11.5px] text-tx-3 max-w-[260px] mb-4 leading-relaxed">
                      Задавайте вопросы по алгоритму, просите подсказки или разбор сложных моментов.
                    </p>

                    <div className="w-full space-y-1.5">
                      {QUICK_PROMPTS.map((prompt) => (
                        <button
                          key={prompt}
                          type="button"
                          disabled={streaming || !aiConfig?.enabled}
                          onClick={() => onSend(prompt)}
                          className="w-full text-left text-[11.5px] bg-bg-2 hover:bg-bg-3 border border-bdr hover:border-brand/40 text-tx-2 hover:text-tx-1 px-3 py-1.5 rounded-lg transition-all disabled:opacity-40 disabled:pointer-events-none cursor-pointer"
                        >
                          {prompt}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {messages.map((msg, i) => (
                  <ChatMessage
                    key={i}
                    message={msg}
                  />
                ))}

                {streaming && (
                  (() => {
                    const parsed = parseThinking(streamingContent);
                    const hasVisibleContent = parsed.content.length > 0;
                    const hasVisibleThinking = (parsed.thinking?.length ?? 0) > 0;

                    if (hasVisibleContent || hasVisibleThinking) {
                      return (
                        <ChatMessage
                          message={{ role: 'assistant', content: streamingContent }}
                          isStreaming
                        />
                      );
                    }

                    return (
                      <motion.div
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        className="flex justify-start items-end gap-2"
                      >
                        <div className="w-6 h-6 rounded-full bg-brand/15 text-brand flex items-center justify-center shrink-0 mb-1 border border-brand/20">
                          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z" />
                            <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z" />
                          </svg>
                        </div>
                        <div className="bg-bg-2 border border-bdr rounded-2xl rounded-bl-sm px-3.5 py-2 shadow-sm flex items-center gap-2">
                          <div className="flex gap-1.5 items-center">
                            <span className="w-1.5 h-1.5 rounded-full bg-brand animate-bounce" style={{ animationDelay: '0ms' }} />
                            <span className="w-1.5 h-1.5 rounded-full bg-brand animate-bounce" style={{ animationDelay: '150ms' }} />
                            <span className="w-1.5 h-1.5 rounded-full bg-brand animate-bounce" style={{ animationDelay: '300ms' }} />
                          </div>
                          <span className="text-[11px] text-tx-3 font-medium">Генерация ответа...</span>
                        </div>
                      </motion.div>
                    );
                  })()
                )}
              </div>

              {/* Input Area */}
              <div className="p-3 border-t border-bdr bg-bg-2/70 shrink-0">
                <div className="relative flex items-end gap-2">
                  <textarea
                    ref={textareaRef}
                    value={input}
                    onChange={(e) => onInputChange(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        if (streaming) {
                          onStop();
                        } else {
                          onSend();
                        }
                      }
                    }}
                    rows={1}
                    placeholder={aiConfig?.enabled ? 'Спросить AI... (Enter для отправки)' : 'AI не настроен'}
                    disabled={!aiConfig?.enabled}
                    className="flex-1 bg-bg-3 border border-bdr rounded-xl px-3 py-2 text-xs sm:text-sm text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none resize-none max-h-28 min-h-[36px] disabled:opacity-50 transition-colors"
                  />

                  {streaming ? (
                    <button
                      type="button"
                      onClick={onStop}
                      className="w-[36px] h-[36px] rounded-xl bg-err hover:bg-err/90 text-white flex items-center justify-center transition-all shrink-0 shadow-sm active:scale-95 animate-pulse cursor-pointer"
                      title="Остановить генерацию"
                    >
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                        <rect x="4" y="4" width="16" height="16" rx="2.5" />
                      </svg>
                    </button>
                  ) : (
                    <button
                      type="button"
                      onClick={() => onSend()}
                      disabled={!input.trim() || !aiConfig?.enabled}
                      className="w-[36px] h-[36px] rounded-xl bg-brand hover:bg-brand-hover disabled:opacity-40 disabled:cursor-not-allowed text-white flex items-center justify-center transition-all shrink-0 shadow-sm active:scale-95 cursor-pointer"
                      title="Отправить сообщение"
                    >
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                        <path d="m22 2-7 20-4-9-9-4z" />
                        <path d="M22 2 11 13" />
                      </svg>
                    </button>
                  )}
                </div>
              </div>
      </div>
    </div>
  );
}

export function AIAssist({
  taskSlug,
  unitSlug,
  taskTitle,
  taskDescription,
  language,
  currentCode,
  templateCode,
  testsCode,
  solutionCode,
  testOutput,
  onDockChange,
  maxDockedWidth,
  resultsOpen,
  resultsCollapsed,
}: {
  taskSlug: string;
  unitSlug: string;
  taskTitle?: string;
  taskDescription?: string;
  language?: string;
  currentCode?: string;
  templateCode?: string;
  testsCode?: string;
  solutionCode?: string;
  testOutput?: string;
  onDockChange?: (docked: boolean, width: number) => void;
  maxDockedWidth?: number | (() => number);
  resultsOpen?: boolean;
  resultsCollapsed?: boolean;
}) {
  const storageKey = `cf_ai_chat_${taskSlug || 'global'}`;

  // Mode state: 'docked' | 'floating' | 'closed' (default: 'closed')
  const [aiMode, setAIModeState] = useState<AIMode>(() => {
    if (typeof window !== 'undefined') {
      try {
        const saved = localStorage.getItem('cf_ai_mode');
        if (saved === 'docked' || saved === 'floating' || saved === 'closed') {
          return saved;
        }
      } catch {
        /* ignore */
      }
    }
    return 'closed';
  });

  const [lastActiveMode, setLastActiveMode] = useState<'docked' | 'floating'>(() => {
    if (typeof window !== 'undefined') {
      try {
        const saved = localStorage.getItem('cf_ai_last_mode');
        if (saved === 'docked' || saved === 'floating') return saved;
      } catch {
        /* ignore */
      }
    }
    return 'docked';
  });

  const setAIMode = useCallback((newMode: AIMode) => {
    setAIModeState(newMode);
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem('cf_ai_mode', newMode);
        if (newMode !== 'closed') {
          setLastActiveMode(newMode);
          localStorage.setItem('cf_ai_last_mode', newMode);
        }
      } catch {
        /* ignore */
      }
    }
  }, []);



  const getMaxWidth = useCallback(() => {
    const fromProp = typeof maxDockedWidth === 'function' ? maxDockedWidth() : maxDockedWidth;
    return typeof fromProp === 'number' ? Math.min(MAX_AI_WIDTH, fromProp) : MAX_AI_WIDTH;
  }, [maxDockedWidth]);

  // Docked resizer hook
  const {
    width: dockedWidth,
    isDragging: isDraggingSplitter,
    splitterProps,
  } = useResizable({
    initialWidth: DEFAULT_AI_WIDTH,
    minWidth: MIN_AI_WIDTH,
    maxWidth: getMaxWidth,
    direction: 'left',
    storageKey: 'cf_ai_width',
  });

  // Notify parent of docked state and width
  useEffect(() => {
    onDockChange?.(aiMode === 'docked', dockedWidth);
  }, [aiMode, dockedWidth, onDockChange]);

  // Floating window draggable & resizable hook
  const {
    position: floatingPos,
    size: floatingSize,
    dragHandleProps,
    getResizeHandleProps,
    resetSize: resetFloatingSize,
  } = useDraggable({
    storageKey: 'cf_ai_floating_pos',
    storageKeySize: 'cf_ai_floating_size',
    width: FLOATING_WIDTH,
    height: FLOATING_HEIGHT,
    minWidth: MIN_AI_WIDTH,
    minHeight: 380,
  });

  // Chat message & config states
  const [messages, setMessages] = useState<AIMessage[]>(() => {
    try {
      const saved = localStorage.getItem(`cf_ai_chat_${taskSlug || 'global'}`);
      return saved ? JSON.parse(saved) : [];
    } catch {
      return [];
    }
  });
  const [input, setInput] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [showThinking, setShowThinking] = useState(() => {
    try {
      return localStorage.getItem('cf_ai_show_thinking') !== 'false';
    } catch {
      return true;
    }
  });

  useEffect(() => {
    const handleStorage = (e: StorageEvent) => {
      if (e.key === 'cf_ai_show_thinking') {
        setShowThinking(e.newValue !== 'false');
      }
    };
    const handleThinkingChanged = (e: Event) => {
      const custom = e as CustomEvent<boolean>;
      if (typeof custom.detail === 'boolean') {
        setShowThinking(custom.detail);
      } else {
        try {
          setShowThinking(localStorage.getItem('cf_ai_show_thinking') !== 'false');
        } catch {
          setShowThinking(true);
        }
      }
    };
    window.addEventListener('storage', handleStorage);
    window.addEventListener('cf_ai_thinking_changed', handleThinkingChanged);
    return () => {
      window.removeEventListener('storage', handleStorage);
      window.removeEventListener('cf_ai_thinking_changed', handleThinkingChanged);
    };
  }, []);
  const { data: aiConfigData } = useQuery({
    queryKey: ['ai-config'],
    queryFn: api.aiConfig,
  });
  const aiConfig = aiConfigData ?? null;
  const abortControllerRef = useRef<AbortController | null>(null);

  // Sync messages when taskSlug changes without triggering cascading render in effect
  const [prevStorageKey, setPrevStorageKey] = useState(storageKey);
  if (prevStorageKey !== storageKey) {
    setPrevStorageKey(storageKey);
    let loaded: AIMessage[] = [];
    try {
      const saved = localStorage.getItem(storageKey);
      if (saved) loaded = JSON.parse(saved);
    } catch {
      /* ignore */
    }
    setMessages(loaded);
  }

  const updateMessages = (updater: AIMessage[] | ((prev: AIMessage[]) => AIMessage[])) => {
    setMessages((prev) => {
      const next = typeof updater === 'function' ? updater(prev) : updater;
      try {
        localStorage.setItem(storageKey, JSON.stringify(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  };

  const handleClearChat = () => {
    setMessages([]);
    try {
      localStorage.removeItem(storageKey);
    } catch {
      /* ignore */
    }
  };

  const handleStop = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    setStreaming(false);

    const raw = streamingContent.trim();
    if (raw) {
      const parsed = parseThinking(raw);
      let finalContent = '';
      if (parsed.thinking) {
        finalContent = `<think>${parsed.thinking}</think>\n\n`;
      }
      if (parsed.content.trim()) {
        finalContent += `${parsed.content.trim()}\n\n*[Ответ прерван пользователем]*`;
      } else {
        finalContent += `*[Ответ прерван пользователем]*`;
      }
      updateMessages((prev) => [...prev, { role: 'assistant', content: finalContent }]);
    } else {
      updateMessages((prev) => [...prev, { role: 'assistant', content: '*[Ответ прерван пользователем]*' }]);
    }

    setStreamingContent('');
  };

  const handleSend = async (textToSend?: string) => {
    const content = (textToSend ?? input).trim();
    if (!content || streaming) return;

    const userMsg: AIMessage = { role: 'user', content };
    const allMessages = [...messages, userMsg];

    const contextParts = [
      `[КОНТЕКСТ ЗАДАЧИ]`,
      `Задача: ${taskTitle ? `"${taskTitle}"` : taskSlug} (unit: ${unitSlug}, slug: ${taskSlug})`,
      language ? `Язык программирования: ${language}` : null,
      taskDescription ? `Условие задачи:\n${taskDescription}` : null,
      templateCode?.trim() && templateCode !== currentCode
        ? `Исходный начальный шаблон (template):\n\`\`\`${language || ''}\n${templateCode}\n\`\`\``
        : null,
      currentCode?.trim()
        ? `Текущий код в редакторе пользователя (проанализируй его сразу, не спрашивай что написано):\n\`\`\`${language || ''}\n${currentCode}\n\`\`\``
        : `Код в редакторе: [пока пуст]`,
      testsCode?.trim()
        ? `Код автоматических тестов задачи (используй для понимания проверок, краевых случаев и требований):\n\`\`\`${language || ''}\n${testsCode}\n\`\`\``
        : null,
      solutionCode?.trim()
        ? `Эталонное авторское решение задачи (пользователь уже открыл его, используй для сравнения, но не выдавай как готовый ответ):\n\`\`\`${language || ''}\n${solutionCode}\n\`\`\``
        : null,
      testOutput?.trim()
        ? `Результаты последних тестов:\n\`\`\`\n${testOutput}\n\`\`\``
        : null,
    ].filter(Boolean).join('\n\n');

    const contextMsg: AIMessage = {
      role: 'system',
      content: contextParts,
    };
    const messagesWithContext = [contextMsg, ...allMessages];

    updateMessages(allMessages);
    setInput('');
    setStreaming(true);
    setStreamingContent('');

    const controller = new AbortController();
    abortControllerRef.current = controller;

    let assistantContent = '';
    let hasStreamError = false;

    const isThinkingEnabled = (() => {
      try {
        return localStorage.getItem('cf_ai_show_thinking') !== 'false';
      } catch {
        return showThinking;
      }
    })();

    try {
      await api.aiChatStream(
        messagesWithContext,
        (chunk) => {
          assistantContent += chunk;
          setStreamingContent(assistantContent);
        },
        (err) => {
          hasStreamError = true;
          setStreaming(false);
          updateMessages((prev) => [...prev, { role: 'assistant', content: `<!-- CF_ERROR -->${err}` }]);
        },
        controller.signal,
        isThinkingEnabled,
      );

      if (!controller.signal.aborted && !hasStreamError) {
        const parsed = parseThinking(assistantContent);
        if (!parsed.content && !parsed.thinking) {
          updateMessages((prev) => [
            ...prev,
            {
              role: 'assistant',
              content: `<!-- CF_ERROR -->Провайдер вернул пустой ответ (0 токенов). Возможно, модель сейчас перегружена — попробуйте повторить запрос или выбрать другую модель.`,
            },
          ]);
        } else {
          updateMessages((prev) => [...prev, { role: 'assistant', content: assistantContent }]);
        }
      }
    } catch (err: unknown) {
      if (!controller.signal.aborted) {
        const msg = err instanceof Error ? err.message : 'Ошибка при получении ответа';
        updateMessages((prev) => [...prev, { role: 'assistant', content: `<!-- CF_ERROR -->${msg}` }]);
      }
    } finally {
      setStreaming(false);
      setStreamingContent('');
      abortControllerRef.current = null;
    }
  };

  const panelProps = {
    messages,
    onClearChat: handleClearChat,
    aiConfig,
    input,
    onInputChange: setInput,
    onSend: handleSend,
    onStop: handleStop,
    streaming,
    streamingContent,
  };

  return (
    <>
      {/* 1. DOCKED MODE: Inline flex column in workspace with slide in/out */}
      <AnimatePresence initial={false}>
        {aiMode === 'docked' && (
          <motion.div
            key="docked-ai-panel"
            initial={{ width: 0, opacity: 0 }}
            animate={{ width: dockedWidth + 4, opacity: 1 }}
            exit={{ width: 0, opacity: 0 }}
            transition={
              isDraggingSplitter
                ? { duration: 0 }
                : { type: 'spring', stiffness: 350, damping: 35 }
            }
            className="shrink-0 h-full flex flex-row overflow-hidden relative"
          >
            {/* Draggable Splitter Handle */}
            <div
              {...splitterProps}
              className={clsx(
                'w-1 shrink-0 bg-bdr hover:bg-brand cursor-col-resize transition-colors relative select-none',
                isDraggingSplitter && 'bg-brand shadow-[0_0_8px_rgba(124,58,237,0.5)]'
              )}
              title="Потяните для изменения ширины (двойной клик — сброс)"
            >
              <div className="absolute inset-y-0 -left-1 w-3 z-30 cursor-col-resize" />
            </div>

            {/* Docked Column */}
            <div
              style={{ width: `${dockedWidth}px` }}
              className="shrink-0 h-full flex flex-col bg-bg-1 border-l border-bdr overflow-hidden"
            >
              <AIChatPanel
                mode="docked"
                onModeChange={setAIMode}
                {...panelProps}
              />
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* 2. FLOATING MODE: Draggable popover hover chat with smooth scale/fade exit */}
      {typeof document !== 'undefined' &&
        createPortal(
          <AnimatePresence>
            {aiMode === 'floating' && (
              <div className="pointer-events-none fixed inset-0 z-30 overflow-hidden">
                <motion.div
                  key="floating-ai-dialog"
                  initial={{ opacity: 0, scale: 0.92, y: 14 }}
                  animate={{ opacity: 1, scale: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.92, y: 14 }}
                  transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] as const }}
                  style={{
                    position: 'absolute',
                    left: `${floatingPos.x}px`,
                    top: `${floatingPos.y}px`,
                    width: floatingSize.width,
                    height: floatingSize.height,
                    maxHeight: 'calc(100vh - 24px)',
                    maxWidth: 'calc(100vw - 24px)',
                  }}
                  className="pointer-events-auto flex flex-col rounded-2xl border border-bdr bg-bg-1 shadow-2xl overflow-hidden relative select-none"
                >
                  <AIChatPanel
                    mode="floating"
                    onModeChange={setAIMode}
                    dragHandleProps={dragHandleProps}
                    {...panelProps}
                  />

                  {/* 8-Directional Resize Handles */}
                  {/* Edges */}
                  <div
                    {...getResizeHandleProps('top')}
                    className="absolute top-0 left-4 right-4 h-2 -translate-y-1 cursor-ns-resize z-40"
                    title="Изменить размер"
                  />
                  <div
                    {...getResizeHandleProps('bottom')}
                    className="absolute bottom-0 left-4 right-4 h-2 translate-y-1 cursor-ns-resize z-40"
                    title="Изменить размер"
                  />
                  <div
                    {...getResizeHandleProps('left')}
                    className="absolute top-4 bottom-4 left-0 w-2 -translate-x-1 cursor-ew-resize z-40"
                    title="Изменить размер"
                  />
                  <div
                    {...getResizeHandleProps('right')}
                    className="absolute top-4 bottom-4 right-0 w-2 translate-x-1 cursor-ew-resize z-40"
                    title="Изменить размер"
                  />

                  {/* Corners */}
                  <div
                    {...getResizeHandleProps('top-left')}
                    className="absolute top-0 left-0 w-4 h-4 -translate-x-1 -translate-y-1 cursor-nwse-resize z-50"
                  />
                  <div
                    {...getResizeHandleProps('top-right')}
                    className="absolute top-0 right-0 w-4 h-4 translate-x-1 -translate-y-1 cursor-nesw-resize z-50"
                  />
                  <div
                    {...getResizeHandleProps('bottom-left')}
                    className="absolute bottom-0 left-0 w-4 h-4 -translate-x-1 translate-y-1 cursor-nesw-resize z-50"
                  />
                  <div
                    {...getResizeHandleProps('bottom-right')}
                    onDoubleClick={resetFloatingSize}
                    title="Потяните для изменения размера (двойной клик — сброс)"
                    className="absolute bottom-0 right-0 w-5 h-5 cursor-nwse-resize z-50 flex items-end justify-end p-1 text-tx-3 hover:text-brand transition-colors group"
                  >
                    <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" className="opacity-40 group-hover:opacity-100 transition-opacity">
                      <circle cx="8" cy="8" r="1" />
                      <circle cx="8" cy="4.5" r="1" />
                      <circle cx="4.5" cy="8" r="1" />
                    </svg>
                  </div>
                </motion.div>
              </div>
            )}
          </AnimatePresence>,
          document.body
        )}

      {/* 3. CLOSED MODE: Bottom-right launcher toggle button with spring exit/enter */}
      {typeof document !== 'undefined' &&
        createPortal(
          <AnimatePresence>
            {aiMode === 'closed' && (
              <motion.div
                key="closed-ai-launcher"
                style={{
                  bottom: !resultsOpen ? '24px' : resultsCollapsed ? '62px' : '290px',
                }}
                className="fixed right-6 z-30 pointer-events-auto transition-[bottom] duration-200"
                initial={{ scale: 0, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                exit={{ scale: 0, opacity: 0 }}
                transition={{ type: 'spring', stiffness: 350, damping: 25 }}
              >
                <button
                  type="button"
                  onClick={() => setAIMode(lastActiveMode || 'docked')}
                  className="w-12 h-12 rounded-full bg-brand hover:bg-brand-hover text-white shadow-lg shadow-brand/30 flex items-center justify-center transition-transform hover:scale-105 active:scale-95 cursor-pointer"
                  title="Открыть AI-ассистент"
                >
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z" />
                    <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z" />
                    <line x1="9" y1="10" x2="9.01" strokeWidth="2" />
                    <line x1="15" y1="10" x2="15.01" strokeWidth="2" />
                  </svg>
                </button>
              </motion.div>
            )}
          </AnimatePresence>,
          document.body
        )}
    </>
  );
}
