import { useState, useRef, useEffect, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import type { AIModelItem } from '../api/types';

export const SYSTEM_PROMPT_SHORT = `# Системный промпт: AI-наставник CourseForge (короткая версия)

Ты — AI-наставник CourseForge (платформа для самообучения программированию). В контексте: теория, условие задачи, эталонное решение (не показывать пользователю, кроме Типа C), тест-кейсы, темплейт, текущий код пользователя. Не спрашивай "что ты уже написал" — код уже виден тебе.

## Алгоритм перед ответом

1. В сообщении есть явный вопрос/просьба о помощи? Нет → короткий ответ, без анализа кода и без предложения решения.
2. Вопрос про механику языка/библиотеки, без привязки к задаче → Тип A: отвечай прямо.
3. Вопрос про путь решения этой задачи (баг, "что дальше", "почему не работает") → Тип B: не давай ответ, только подсказка.
4. Прямая просьба дать решение/код → Тип C: уточнение → подтверждение → выдача.
Сомневаешься между B и C — выбирай B.

## Правила по типам

**Нет запроса (приветствие, смолток и т.п.):** ответь коротко и по-человечески. Если диалог уже начат — не здоровайся повторно ("Привет!") и не повторяй каждый раз название задачи, отвечай живо и естественно. Не анализируй код, не указывай баги, даже если видишь их. Темплейт/несвязный набросок — не баг, а стартовая точка, не критикуй его без запроса.

**Тип A (механика языка):** отвечай сразу и полно, это общее знание, не подсказка к задаче.

**Тип B (путь решения):** не давай готовый ответ. Формат: (1) где проблема — строка/фрагмент; (2) категория проблемы (гонка данных, off-by-one, неверное условие и т.п.) и почему это проблема; (3) наводящий вопрос без готового исправления. То же самое, если пользователь просто прислал код с вопросом "проверь"/"почему не работает" — без явной просьбы решения это всё ещё Тип B.

**Тип C (прямой запрос решения):** сначала спроси: "Точно нужно решение целиком? Могу разобрать по шагам — так эффективнее для обучения. Что выбираешь: целиком или по частям?"
- "По частям" → веди пошагово, указывай фрагмент и направление мысли, не давай финальный код куска, пока пользователь не попробовал сам.
- Подтвердил "целиком" → дай полный код с краткими комментариями логики + скажи, что для закрепления полезно переписать руками.

## Формат ответа
Ёмко, структурировано (списки/заголовки при нескольких пунктах), разделяй "что не так" и "куда думать", всегда на русском.

## Запрещено всегда
- Показывать эталонное решение или дословные тест-кейсы без подтверждения по Типу C.
- Начинать с готового кода-решения без запроса и подтверждения.
- Анализировать код или предлагать решение без явного запроса.
- Придумывать детали, которых нет в контексте.
- Спрашивать про прогресс, если код уже передан.`;

export const SYSTEM_PROMPT_FULL = `# Системный промпт: AI-наставник CourseForge

## Роль и контекст

Ты — AI-наставник платформы CourseForge (платформа для самообучения программированию: курсы → задачи, каждая задача содержит теорию, условие, эталонное решение, тест-кейсы и темплейт).

В каждом сообщении тебе передаётся:
- **Теория** к текущей задаче
- **Условие** задачи
- **Эталонное решение** — только для твоей ориентировки, НИКОГДА не показывай и не пересказывай его пользователю, кроме случая из раздела "Тип C" ниже
- **Тест-кейсы**
- **Темплейт** (стартовая заготовка)
- **Текущий код пользователя** из редактора

Этот контекст у тебя уже есть. Никогда не спрашивай пользователя "что ты уже написал", "на каком ты этапе", "покажи код" — если код передан, работай с ним напрямую.

---

## Алгоритм перед каждым ответом (выполняй по порядку)

1. **В сообщении пользователя есть явный вопрос или просьба о помощи?**
   Нет (приветствие / реплика без запроса) → см. блок "Реактивность". Не анализируй код, не указывай на баги, не предлагай решение.

2. **Вопрос касается только механики языка/библиотеки, без привязки к конкретной задаче?** → Тип A. Отвечай прямо.

3. **Вопрос касается пути решения именно этой задачи (баг, "что дальше", "почему не работает", "какой алгоритм")?** → Тип B. Сократический метод, без готового ответа.

4. **Пользователь прямо просит решение/код целиком?** → Тип C. Уточнение → подтверждение → выдача (с приоритетом на "по частям").

Если сомневаешься между B и C — трактуй как B (подсказка, не решение).

---

## Реактивность: ты не действуешь по своей инициативе

Ты не автор кода, а помощник, реагирующий на запрос. Пока пользователь явно не попросил разобрать код или подсказать — не трогай тему решения, даже если видишь баги.

**Приветствие / реплика без запроса / смолток** ("привет", "как дела", "здарова", "я тут", "норм?") → короткий человеческий ответ + готовность помочь. Никакого непрошенного разбора кода.
- Если это **первое** сообщение в диалоге: коротко поздоровайся и предложи помощь («Привет! Вижу, работаешь над задачей «...». Если есть вопрос — пиши.»).
- Если диалог **уже начат** (в истории уже было приветствие): не здоровайся заново («Привет!»), не повторяй шаблонно название задачи, а отвечай живо и естественно на реплику пользователя (например, на «Как дела» → «Всё отлично, готов помочь! Какой у тебя вопрос по задаче?»).

**Темплейт-заготовка или явно несвязный/некомпилируемый набросок** → это стартовая точка, а не ошибка. Не критикуй её как баг, пока не спросили конкретно.

---

## Тип A — Вопрос о механике языка/библиотеки

Примеры: «как работает срез в Go», «что делает functools.reduce», «чем отличается deep copy от shallow copy», «как объявить мьютекс», «какая сложность у sort.Slice».

Это НЕ подсказка к решению задачи — общее знание языка. Отвечай сразу и полно, при необходимости на нейтральном примере, не связанном с условием задачи.

---

## Тип B — Вопрос про путь решения этой задачи

Примеры: «почему тест падает», «в чём тут баг», «что делать дальше», «какой алгоритм тут нужен».

НЕ давай готовый ответ и не пиши код-исправление. Структура ответа:
1. **Где** проблема — конкретная строка/фрагмент/переменная.
2. **Что за категория** проблемы (гонка данных, незалоченный мьютекс, off-by-one, неверное условие выхода, неверная сложность и т.п.) и **почему** это проблема.
3. **Наводящий вопрос** или указание, о какой концепции подумать — без формулировки готового исправления.

> Пользователь: «Почему тест падает?»
> Ты: «Смотри на \`counter++\` в горутине (строка 14) — несколько горутин пишут в переменную без синхронизации, это гонка данных. Какой примитив в Go позволяет безопасно инкрементировать значение из разных потоков?»

Это же правило действует, если пользователь просто прислал код с вопросом «проверь» / «почему не работает» — без явного запроса решения ты всё равно остаёшься в Типе B, а не переходишь к готовому фиксу.

---

## Тип C — Прямой запрос решения («дай решение», «напиши код», «просто реши»)

Не выдавай решение сразу. Сначала уточни:

> «Точно нужно готовое решение целиком? Обычно эффективнее разобрать по шагам — я укажу, что не так, кусок за куском, а ты допишешь логику сам. Что выбираешь: (1) решение целиком, (2) разбор по частям?»

- **Пользователь выбрал «по частям»** → веди пошагово: указывай на конкретный фрагмент, что там не так и в какую сторону думать. Не давай финальный код фрагмента, пока пользователь не попробовал сам хотя бы раз.
- **Пользователь подтвердил, что нужно решение целиком** → можешь дать полный код, но:
  - сопроводи краткими комментариями, объясняющими логику (не просто дамп кода);
  - явно скажи, что для закрепления материала полезнее переписать его руками, а не копировать.

---

## Формат ответа

- Ёмко, без воды.
- Структурируй списками/заголовками, если пунктов несколько.
- Разделяй «что не так» и «куда думать» — не сливай в один абзац.
- Всегда на русском языке.

## Запрещено всегда

- Показывать эталонное решение или дословные тест-кейсы, если это не оговорено явно выше (Тип C после подтверждения).
- Начинать ответ с готового кода-решения без подтверждения пользователя.
- Анализировать код или предлагать решение без явного запроса пользователя.
- Придумывать детали условия/тестов, которых нет в переданном контексте.
- Спрашивать «что ты уже написал», если код уже есть в контексте.`;

interface ProviderPreset {
  id: string;
  label: string;
  defaultUrl: string;
  apiFormat: 'openai' | 'anthropic';
}

const PROVIDER_PRESETS: ProviderPreset[] = [
  { id: 'freetoken', label: 'FreeToken (Локальный)', defaultUrl: 'http://localhost:1919/v1', apiFormat: 'openai' },
  { id: 'ollama', label: 'Ollama (Локальный)', defaultUrl: 'http://localhost:11434/v1', apiFormat: 'openai' },
  { id: 'openrouter', label: 'OpenRouter', defaultUrl: 'https://openrouter.ai/api/v1', apiFormat: 'openai' },
  { id: 'openai', label: 'OpenAI', defaultUrl: 'https://api.openai.com/v1', apiFormat: 'openai' },
  { id: 'anthropic', label: 'Anthropic Claude', defaultUrl: 'https://api.anthropic.com/v1', apiFormat: 'anthropic' },
  { id: 'deepseek', label: 'DeepSeek', defaultUrl: 'https://api.deepseek.com/v1', apiFormat: 'openai' },
  { id: 'custom', label: 'Кастомный провайдер (Custom)', defaultUrl: '', apiFormat: 'openai' },
];

function ProviderSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (val: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const currentLabel = PROVIDER_PRESETS.find((o) => o.id === value)?.label || value;

  return (
    <div ref={rootRef} className="relative w-full">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center justify-between gap-2 rounded-xl border border-bdr bg-bg-3 px-3 py-2 text-xs sm:text-sm text-tx-1 hover:bg-bg-4 focus:border-brand focus:ring-1 focus:ring-brand transition-colors text-left outline-none cursor-pointer"
      >
        <span className="truncate flex-1 font-medium">{currentLabel}</span>
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={clsx('text-tx-3 transition-transform shrink-0', open && 'rotate-180')}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {open && (
        <div className="absolute left-0 right-0 z-50 mt-1.5 max-h-64 overflow-y-auto rounded-xl border border-bdr bg-bg-2 py-1 shadow-2xl">
          {PROVIDER_PRESETS.map((opt) => {
            const isSelected = opt.id === value;
            return (
              <button
                key={opt.id}
                type="button"
                onClick={() => {
                  onChange(opt.id);
                  setOpen(false);
                }}
                className={clsx(
                  'flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-xs sm:text-sm transition-colors cursor-pointer',
                  isSelected
                    ? 'bg-brand/15 text-brand font-semibold'
                    : 'text-tx-1 hover:bg-bg-3'
                )}
              >
                <div className="flex flex-col min-w-0 flex-1">
                  <span className="truncate">{opt.label}</span>
                  {opt.defaultUrl && (
                    <span className="text-[10px] text-tx-3 font-mono truncate">{opt.defaultUrl}</span>
                  )}
                </div>
                {isSelected && (
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-brand">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function ApiStandardSelect({
  value,
  onChange,
}: {
  value: 'openai' | 'anthropic';
  onChange: (val: 'openai' | 'anthropic') => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const standards: { value: 'openai' | 'anthropic'; label: string; desc: string }[] = [
    { value: 'openai', label: 'OpenAI-совместимый', desc: 'Chat Completions (/v1/chat/completions)' },
    { value: 'anthropic', label: 'Anthropic Claude', desc: 'Messages API (/v1/messages)' },
  ];

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const currentLabel = standards.find((s) => s.value === value)?.label || value;

  return (
    <div ref={rootRef} className="relative w-full">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center justify-between gap-2 rounded-xl border border-bdr bg-bg-3 px-3 py-2 text-xs sm:text-sm text-tx-1 hover:bg-bg-4 focus:border-brand focus:ring-1 focus:ring-brand transition-colors text-left outline-none cursor-pointer"
      >
        <span className="truncate flex-1 font-medium">{currentLabel}</span>
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={clsx('text-tx-3 transition-transform shrink-0', open && 'rotate-180')}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {open && (
        <div className="absolute left-0 right-0 z-50 mt-1.5 max-h-60 overflow-y-auto rounded-xl border border-bdr bg-bg-2 py-1 shadow-2xl">
          {standards.map((s) => {
            const isSelected = s.value === value;
            return (
              <button
                key={s.value}
                type="button"
                onClick={() => {
                  onChange(s.value);
                  setOpen(false);
                }}
                className={clsx(
                  'flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-xs sm:text-sm transition-colors cursor-pointer',
                  isSelected
                    ? 'bg-brand/15 text-brand font-semibold'
                    : 'text-tx-1 hover:bg-bg-3'
                )}
              >
                <div className="flex flex-col min-w-0 flex-1">
                  <span className="truncate">{s.label}</span>
                  <span className="text-[10px] text-tx-3 font-mono truncate">{s.desc}</span>
                </div>
                {isSelected && (
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-brand">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function ModelSelect({
  models,
  value,
  onChange,
  disabled,
  placeholder,
}: {
  models: AIModelItem[];
  value: string;
  onChange: (model: string) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) setSearchQuery('');
  }

  useEffect(() => {
    if (!open) return;
    const timer = setTimeout(() => {
      inputRef.current?.focus();
    }, 50);

    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const isDisabled = disabled || models.length === 0;
  const selectedModel = models.find((m) => m.id === value);

  const filtered = models.filter((m) =>
    m.id.toLowerCase().includes(searchQuery.toLowerCase().trim())
  );

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (filtered.length > 0) {
        onChange(filtered[0].id);
        setOpen(false);
      }
    }
  };

  return (
    <div ref={rootRef} className="relative w-full">
      <button
        type="button"
        disabled={isDisabled}
        onClick={() => !isDisabled && setOpen((v) => !v)}
        className={clsx(
          'w-full flex items-center justify-between gap-2 rounded-xl border border-bdr bg-bg-3 px-3 py-2 text-xs sm:text-sm transition-colors text-left font-mono outline-none',
          isDisabled
            ? 'opacity-40 cursor-not-allowed text-tx-3 select-none'
            : open
              ? 'border-brand ring-1 ring-brand text-tx-1'
              : 'text-tx-1 hover:bg-bg-4 cursor-pointer'
        )}
      >
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <span className={clsx('truncate font-medium', !value && 'text-tx-3')}>
            {value || (isDisabled ? (placeholder || 'Модели не загружены') : 'Выберите модель...')}
          </span>
          {selectedModel && !selectedModel.available && (
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-err/10 text-err/80 border border-err/20 shrink-0 font-sans">
              недоступна
            </span>
          )}
        </div>
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={clsx('text-tx-3 transition-transform shrink-0', open && 'rotate-180')}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {open && !isDisabled && (
        <div className="absolute left-0 right-0 z-50 mt-1.5 flex flex-col rounded-xl border border-bdr bg-bg-2 shadow-2xl overflow-hidden max-h-72">
          <div className="p-2 pb-1 bg-bg-2 shrink-0">
            <div className="relative flex items-center">
              <svg
                width="13"
                height="13"
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
                ref={inputRef}
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Поиск модели..."
                autoComplete="off"
                className="w-full bg-bg-3 border border-bdr/80 rounded-lg pl-8 pr-7 py-1.5 text-xs text-tx-1 placeholder:text-tx-3/60 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono transition-colors"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => {
                    setSearchQuery('');
                    inputRef.current?.focus();
                  }}
                  className="absolute right-2 text-tx-3 hover:text-tx-1 p-0.5 cursor-pointer"
                >
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2">
                    <path d="M18 6 6 18M6 6l12 12" />
                  </svg>
                </button>
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto max-h-60 py-1">
            {filtered.length === 0 ? (
              <div className="px-3 py-4 text-center text-xs text-tx-3">
                Ничего не найдено
              </div>
            ) : (
              filtered.map((m) => {
                const isSelected = m.id === value;
                return (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => {
                      onChange(m.id);
                      setOpen(false);
                    }}
                    className={clsx(
                      'flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-xs sm:text-sm font-mono transition-colors cursor-pointer',
                      isSelected
                        ? 'bg-brand/15 text-brand font-semibold'
                        : 'text-tx-1 hover:bg-bg-3'
                    )}
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <span className="truncate">{m.id}</span>
                      {!m.available && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-err/10 text-err/80 border border-err/20 shrink-0 font-sans">
                          недоступна
                        </span>
                      )}
                    </div>
                    {isSelected && (
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-brand">
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

export function AISettingsSection() {
  const queryClient = useQueryClient();

  const [providerPreset, setProviderPreset] = useState('freetoken');
  const [apiFormat, setApiFormat] = useState<'openai' | 'anthropic'>('openai');
  const [baseURL, setBaseURL] = useState('http://localhost:1919/v1');
  const [apiKey, setApiKey] = useState('');
  const [showApiKey, setShowApiKey] = useState(false);
  const [hasExistingKey, setHasExistingKey] = useState(false);
  const [model, setModel] = useState('');
  const [availableModels, setAvailableModels] = useState<AIModelItem[]>([]);
  const [fetchingModels, setFetchingModels] = useState(false);
  const [checkingAvailability, setCheckingAvailability] = useState(false);
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [systemPrompt, setSystemPrompt] = useState(SYSTEM_PROMPT_FULL);
  const [thinking, setThinking] = useState(() => {
    try {
      return localStorage.getItem('cf_ai_show_thinking') !== 'false';
    } catch {
      return true;
    }
  });
  const [loadingConfig, setLoadingConfig] = useState(true);

  const handleToggleThinking = (next: boolean) => {
    setThinking(next);
    try {
      localStorage.setItem('cf_ai_show_thinking', String(next));
      window.dispatchEvent(new CustomEvent('cf_ai_thinking_changed', { detail: next }));
    } catch {
      /* ignore */
    }
  };

  const [isSaving, setIsSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [globalError, setGlobalError] = useState<string | null>(null);

  const isCustom = providerPreset === 'custom';
  const isApiKeyRequiredForPreset = (presetId: string) =>
    ['openrouter', 'openai', 'anthropic', 'deepseek'].includes(presetId);
  const isKeyRequired = isApiKeyRequiredForPreset(providerPreset);

  const fetchModels = useCallback(
    async (
      fmt: 'openai' | 'anthropic',
      url: string,
      key?: string,
      checkAvail: boolean = false,
      presetId?: string
    ) => {
      if (!url.trim()) return;

      const activePreset = presetId ?? providerPreset;
      const keyRequired = isApiKeyRequiredForPreset(activePreset);
      const effectiveKey = (key !== undefined ? key : apiKey).trim();

      if (keyRequired && !effectiveKey && !hasExistingKey) {
        setAvailableModels([]);
        setModelsError(null);
        return;
      }

      if (checkAvail) {
        setCheckingAvailability(true);
      } else {
        setFetchingModels(true);
      }
      setModelsError(null);
      try {
        const list = await api.aiModels({
          provider: fmt,
          base_url: url.trim(),
          api_key: effectiveKey,
          check_availability: checkAvail,
        });
        setAvailableModels(list);
        if (list.length > 0) {
          setModel((curr) => (curr && list.some((m: AIModelItem) => m.id === curr) ? curr : list[0].id));
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : 'Не удалось получить список моделей';
        setModelsError(msg);
        setAvailableModels([]);
      } finally {
        setFetchingModels(false);
        setCheckingAvailability(false);
      }
    },
    [apiKey, hasExistingKey, providerPreset]
  );

  useEffect(() => {
    api.aiConfig()
      .then((cfg) => {
        if (cfg) {
          setBaseURL(cfg.base_url || 'http://localhost:1919/v1');
          if (cfg.api_key) {
            setApiKey(cfg.api_key);
            setHasExistingKey(true);
          } else {
            setApiKey('');
            setHasExistingKey(false);
          }
          setModel(cfg.model || '');
          const prov = cfg.provider === 'anthropic' ? 'anthropic' : 'openai';
          setApiFormat(prov);
          setSystemPrompt(cfg.system_prompt || SYSTEM_PROMPT_FULL);

          const matchedPreset = PROVIDER_PRESETS.find(
            (p) => p.id !== 'custom' && cfg.base_url?.toLowerCase().includes(p.id)
          );
          const activePresetId = matchedPreset ? matchedPreset.id : 'custom';
          setProviderPreset(activePresetId);

          const keyRequired = isApiKeyRequiredForPreset(activePresetId);
          if (!keyRequired || cfg.api_key) {
            fetchModels(prov, cfg.base_url || 'http://localhost:1919/v1', cfg.api_key, false, activePresetId);
          } else {
            setAvailableModels([]);
            setModelsError(null);
          }
        }
      })
      .catch(() => {
        fetchModels('openai', 'http://localhost:1919/v1', '', false, 'freetoken');
      })
      .finally(() => {
        setLoadingConfig(false);
      });
  }, [fetchModels]);

  const handleProviderChange = (presetId: string) => {
    setProviderPreset(presetId);
    const preset = PROVIDER_PRESETS.find((p) => p.id === presetId);
    if (preset && preset.id !== 'custom') {
      setBaseURL(preset.defaultUrl);
      setApiFormat(preset.apiFormat);
      setApiKey('');
      setHasExistingKey(false);
      setModelsError(null);
      setAvailableModels([]);
      setModel('');

      const keyRequired = isApiKeyRequiredForPreset(presetId);
      if (!keyRequired) {
        fetchModels(preset.apiFormat, preset.defaultUrl, '', false, presetId);
      }
    } else if (presetId === 'custom') {
      setModelsError(null);
      if (baseURL.trim()) {
        fetchModels(apiFormat, baseURL, apiKey, false, 'custom');
      }
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    setSaveSuccess(false);
    setGlobalError(null);
    try {
      await api.aiSaveConfig({
        provider: apiFormat,
        base_url: baseURL.trim(),
        api_key: apiKey.trim() || (hasExistingKey ? '********' : ''),
        model,
        system_prompt: systemPrompt,
      });
      await queryClient.invalidateQueries({ queryKey: ['ai-config'] });
      try {
        localStorage.setItem('cf_ai_show_thinking', String(thinking));
        window.dispatchEvent(new CustomEvent('cf_ai_thinking_changed', { detail: thinking }));
      } catch {
        /* ignore */
      }
      setSaveSuccess(true);
      if (apiKey.trim()) {
        setHasExistingKey(true);
      }
      setTimeout(() => setSaveSuccess(false), 4000);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Не удалось сохранить настройки';
      setGlobalError(msg);
    } finally {
      setIsSaving(false);
    }
  };

  if (loadingConfig) {
    return (
      <div className="flex items-center justify-center p-12 text-tx-3 text-sm">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="animate-spin mr-2">
          <path d="M21 12a9 9 0 1 1-6.219-8.56" />
        </svg>
        <span>Загрузка настроек AI...</span>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Notifications */}
      {saveSuccess && (
        <div className="rounded-xl border border-ok/30 bg-ok/10 p-3 flex items-center gap-2 text-xs font-medium text-ok">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
          <span>Настройки AI успешно сохранены и применены</span>
        </div>
      )}

      {globalError && (
        <div className="rounded-xl border border-err/30 bg-err/10 p-3 flex flex-col gap-1.5 overflow-hidden">
          <div className="flex items-center gap-1.5 text-xs font-semibold text-err">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <span>Ошибка сохранения</span>
          </div>
          <div className="text-[11px] font-mono text-err/90 whitespace-pre-wrap select-text bg-bg-1/40 rounded-lg p-2 border border-err/15">
            {globalError}
          </div>
        </div>
      )}

      {/* Connection Section */}
      <div className="space-y-4">
        <div>
          <h3 className="text-sm font-semibold text-tx-1">Подключение к AI-модели</h3>
          <p className="text-xs text-tx-3 mt-0.5">
            Выберите готовый сервис (OpenRouter, OpenAI, Claude, DeepSeek) или локальный сервер (Ollama, FreeToken)
          </p>
        </div>

        <div className="rounded-xl border border-bdr bg-bg-2/50 p-4 space-y-4">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-tx-2">Провайдер</label>
            <ProviderSelect value={providerPreset} onChange={handleProviderChange} />
          </div>

          {isCustom && (
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-tx-2">Стандарт API</label>
              <ApiStandardSelect
                value={apiFormat}
                onChange={(fmt) => {
                  setApiFormat(fmt);
                  if (apiKey.trim() || !isKeyRequired || hasExistingKey) {
                    fetchModels(fmt, baseURL, apiKey, false);
                  }
                }}
              />
            </div>
          )}

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-tx-2">Base URL</label>
            <input
              type="text"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              onBlur={() => {
                if (apiKey.trim() || !isKeyRequired || hasExistingKey) {
                  fetchModels(apiFormat, baseURL, apiKey, false);
                }
              }}
              placeholder="https://api.openai.com/v1"
              className="w-full bg-bg-3 border border-bdr rounded-lg px-3 py-2 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono transition-colors"
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <label className="text-xs font-medium text-tx-2">API Key</label>
                {isKeyRequired && (
                  <span className="text-[10px] text-warn bg-warn/10 px-1.5 py-0.5 rounded border border-warn/20 font-medium">
                    обязательно
                  </span>
                )}
              </div>
              {hasExistingKey && (
                <span className="text-[11px] text-ok flex items-center gap-1 font-medium">
                  <span>✓ Ключ сохранён</span>
                </span>
              )}
            </div>
            <div className="relative flex items-center">
              <input
                type={showApiKey ? 'text' : 'password'}
                value={apiKey}
                onChange={(e) => {
                  const val = e.target.value;
                  setApiKey(val);
                  setHasExistingKey(false);
                  if (!val.trim() && isKeyRequired) {
                    setAvailableModels([]);
                    setModelsError(null);
                  }
                }}
                onBlur={() => {
                  if (apiKey.trim() || !isKeyRequired) {
                    fetchModels(apiFormat, baseURL, apiKey.trim(), false);
                  }
                }}
                placeholder={hasExistingKey ? '•••••••••••••••• (не изменять)' : isKeyRequired ? 'sk-... (обязательно для загрузки моделей)' : 'Не требуется'}
                className="w-full bg-bg-3 border border-bdr rounded-lg pl-3 pr-9 py-2 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono transition-colors"
              />
              <button
                type="button"
                onClick={() => setShowApiKey((v) => !v)}
                className="absolute right-2 text-tx-3 hover:text-tx-1 p-1 rounded transition-colors cursor-pointer"
                title={showApiKey ? 'Скрыть API ключ' : 'Показать API ключ'}
              >
                {showApiKey ? (
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
                    <line x1="1" y1="1" x2="23" y2="23" />
                  </svg>
                ) : (
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                )}
              </button>
            </div>
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-tx-2">Модель</label>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => fetchModels(apiFormat, baseURL, apiKey, true)}
                  disabled={checkingAvailability || fetchingModels || !baseURL.trim() || (isKeyRequired && !apiKey.trim() && !hasExistingKey)}
                  className="text-[11px] text-brand hover:underline flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer font-medium"
                  title={isKeyRequired && !apiKey.trim() && !hasExistingKey ? 'Сначала укажите API-ключ' : 'Проверить доступность моделей'}
                >
                  <svg
                    width="12"
                    height="12"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className={clsx(checkingAvailability && 'animate-spin')}
                  >
                    {checkingAvailability ? (
                      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
                    ) : (
                      <>
                        <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                        <polyline points="22 4 12 14.01 9 11.01" />
                      </>
                    )}
                  </svg>
                  <span>{checkingAvailability ? 'Проверка...' : 'Проверить доступность'}</span>
                </button>
                <span className="text-tx-3/40 text-[10px]">·</span>
                <button
                  type="button"
                  onClick={() => fetchModels(apiFormat, baseURL, apiKey, false)}
                  disabled={fetchingModels || checkingAvailability || !baseURL.trim() || (isKeyRequired && !apiKey.trim() && !hasExistingKey)}
                  className="text-[11px] text-tx-3 hover:text-tx-1 flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
                  title={isKeyRequired && !apiKey.trim() && !hasExistingKey ? 'Сначала укажите API-ключ' : 'Обновить список моделей'}
                >
                  <svg
                    width="11"
                    height="11"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className={clsx(fetchingModels && 'animate-spin')}
                  >
                    <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
                    <path d="M3 3v5h5" />
                    <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
                    <path d="M16 21h5v-5" />
                  </svg>
                  <span>{fetchingModels ? 'Загрузка...' : 'Обновить'}</span>
                </button>
              </div>
            </div>
            <ModelSelect
              models={availableModels}
              value={model}
              onChange={setModel}
              disabled={fetchingModels || checkingAvailability || (isKeyRequired && !apiKey.trim() && !hasExistingKey)}
              placeholder={
                isKeyRequired && !apiKey.trim() && !hasExistingKey
                  ? 'Сначала укажите API-ключ...'
                  : fetchingModels || checkingAvailability
                    ? 'Загрузка моделей...'
                    : modelsError
                      ? modelsError
                      : 'Сначала укажите Base URL...'
              }
            />

            {modelsError && (
              <div className="rounded-xl border border-err/30 bg-err/10 p-3 flex flex-col gap-1.5 mt-2 overflow-hidden">
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 text-xs font-semibold text-err">
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0">
                      <circle cx="12" cy="12" r="10" />
                      <line x1="12" y1="8" x2="12" y2="12" />
                      <line x1="12" y1="16" x2="12.01" y2="16" />
                    </svg>
                    <span className="truncate">Ошибка загрузки моделей</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => fetchModels(apiFormat, baseURL, apiKey, false)}
                    className="px-2 py-0.5 rounded-md bg-err/15 hover:bg-err/25 text-err text-[11px] font-medium border border-err/30 transition-colors shrink-0 flex items-center gap-1 cursor-pointer select-none active:scale-95"
                    title="Повторить запрос моделей"
                  >
                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
                      <path d="M3 3v5h5" />
                      <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
                      <path d="M16 21h5v-5" />
                    </svg>
                    <span>Повторить</span>
                  </button>
                </div>
                <div className="max-h-28 overflow-y-auto pr-1 text-[11px] font-mono leading-relaxed text-err/90 break-all whitespace-pre-wrap select-text bg-bg-1/40 rounded-lg p-2 border border-err/15">
                  {modelsError}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Thinking Mode Section */}
      <div className="space-y-2">
        <div>
          <h3 className="text-sm font-semibold text-tx-1">Режим мышления</h3>
          <p className="text-xs text-tx-3 mt-0.5">Управление передачей параметров цепочки рассуждений (reasoning) в модель</p>
        </div>
        <div className="rounded-xl border border-bdr bg-bg-2/50 p-4">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-3 min-w-0 flex-1">
              <div className="w-8 h-8 rounded-lg bg-brand/10 text-brand border border-brand/20 flex items-center justify-center shrink-0 mt-0.5">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 4.44-2.04" />
                  <path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-4.44-2.04" />
                </svg>
              </div>
              <div className="space-y-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-semibold text-tx-1">Включить мышление</span>
                  <span
                    className={clsx(
                      'text-[10px] px-1.5 py-0.5 rounded font-medium border',
                      thinking
                        ? 'bg-brand/10 text-brand border-brand/20'
                        : 'bg-bg-3 text-tx-3 border-bdr'
                    )}
                  >
                    {thinking ? 'Включено' : 'Выключено'}
                  </span>
                </div>
                <p className="text-xs text-tx-3 leading-relaxed">
                  Включает передачу параметров мышления (reasoning) в модель при запросе. Работает не для всех моделей: из-за архитектурных ограничений некоторые модели будут думать, если мышление выключено, и не думать, если включено.
                </p>
              </div>
            </div>

            <button
              type="button"
              role="switch"
              aria-checked={thinking}
              onClick={() => handleToggleThinking(!thinking)}
              className={clsx(
                'relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-brand focus:ring-offset-2 focus:ring-offset-bg-2 select-none',
                thinking ? 'bg-brand' : 'bg-bg-4'
              )}
              title={thinking ? 'Отключить режим мышления' : 'Включить режим мышления'}
            >
              <span
                aria-hidden="true"
                className={clsx(
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out',
                  thinking ? 'translate-x-5' : 'translate-x-0'
                )}
              />
            </button>
          </div>
        </div>
      </div>

      {/* System Prompt Section */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-tx-1">Системный промпт</h3>
            <p className="text-xs text-tx-3 mt-0.5">Инструкции по стилю подсказок, тону и запретам для AI-наставника</p>
          </div>
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => setSystemPrompt(SYSTEM_PROMPT_FULL)}
              className={clsx(
                'px-2.5 py-1 text-xs rounded-lg border transition-colors font-medium select-none cursor-pointer',
                systemPrompt.trim() === SYSTEM_PROMPT_FULL.trim()
                  ? 'bg-brand/20 text-brand border-brand/40 shadow-xs'
                  : 'bg-bg-3 hover:bg-bg-4 text-tx-3 hover:text-tx-1 border-bdr'
              )}
            >
              Полный
            </button>
            <button
              type="button"
              onClick={() => setSystemPrompt(SYSTEM_PROMPT_SHORT)}
              className={clsx(
                'px-2.5 py-1 text-xs rounded-lg border transition-colors font-medium select-none cursor-pointer',
                systemPrompt.trim() === SYSTEM_PROMPT_SHORT.trim()
                  ? 'bg-brand/20 text-brand border-brand/40 shadow-xs'
                  : 'bg-bg-3 hover:bg-bg-4 text-tx-3 hover:text-tx-1 border-bdr'
              )}
            >
              Короткий
            </button>
          </div>
        </div>
        <textarea
          value={systemPrompt}
          onChange={(e) => setSystemPrompt(e.target.value)}
          rows={10}
          className="w-full bg-bg-2/50 border border-bdr rounded-xl p-3.5 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none resize-y transition-colors leading-relaxed font-mono"
          placeholder="Инструкция для поведения AI..."
        />
      </div>

      {/* Save Button */}
      <div className="pt-2 flex justify-end">
        <button
          type="button"
          onClick={handleSave}
          disabled={isSaving || !model || availableModels.length === 0}
          className="px-6 py-2.5 bg-brand hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl transition-all shadow-md shadow-brand/20 active:scale-[0.98] flex items-center justify-center gap-2 cursor-pointer"
        >
          {isSaving ? (
            <>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="animate-spin">
                <path d="M21 12a9 9 0 1 1-6.219-8.56" />
              </svg>
              <span>Сохранение...</span>
            </>
          ) : (
            <span>Сохранить настройки</span>
          )}
        </button>
      </div>
    </div>
  );
}
