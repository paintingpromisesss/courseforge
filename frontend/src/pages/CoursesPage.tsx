import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { api } from '../api/client';

function FolderIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-1 14H6L5 6" />
      <path d="M10 11v6M14 11v6" />
      <path d="M9 6V4h6v2" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function PencilIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 20h9" />
      <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

const catKey = (slug: string) => `cat:${slug}`;
const crsKey = (slug: string) => `crs:${slug}`;

// "Thanos snap" — card dissolves: blurs, drifts up, scales out and fades, with a
// small per-card delay so a batch deletion cascades into dust instead of vanishing.
const dustExit = (i: number) => ({
  opacity: 0,
  scale: 1.25,
  filter: 'blur(10px)',
  y: -10,
  transition: { duration: 0.3, delay: i * 0.04, ease: 'easeOut' as const },
});

function SelectMark({ on }: { on: boolean }) {
  return (
    <span
      className={clsx(
        'absolute top-3 right-3 w-5 h-5 rounded-full border flex items-center justify-center transition-colors',
        on ? 'bg-err border-err text-white' : 'bg-bg-1 border-bdr text-transparent',
      )}
    >
      <CheckIcon />
    </span>
  );
}

export function CoursesPage() {
  const qc = useQueryClient();
  const [editMode, setEditMode] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [purge, setPurge] = useState(false);
  const [creating, setCreating] = useState(false);

  const { data: catalogs, isLoading: catsLoading } = useQuery({
    queryKey: ['catalogs'],
    queryFn: api.listCatalogs,
  });
  const { data: courses, isLoading: coursesLoading } = useQuery({
    queryKey: ['courses'],
    queryFn: api.listCourses,
  });

  const exitMode = () => { setEditMode(false); setSelected(new Set()); setConfirmOpen(false); setPurge(false); setCreating(false); };

  const createMut = useMutation({
    mutationFn: (body: { title: string; description: string }) => api.createCatalog(body),
    onSuccess: () => {
      setCreating(false);
      qc.invalidateQueries({ queryKey: ['catalogs'] });
    },
  });

  const toggle = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
  };

  const delMut = useMutation({
    mutationFn: async (purgeCourses: boolean) => {
      const results = await Promise.allSettled(
        [...selected].map((key) => {
          const slug = key.slice(4);
          return key.startsWith('cat:') ? api.deleteCatalog(slug, purgeCourses) : api.deleteCourse(slug);
        }),
      );
      const failed = results.filter((r) => r.status === 'rejected').length;
      if (failed) throw new Error(`Не удалось удалить: ${failed} из ${results.length}`);
    },
    // always refetch so the UI reflects what was actually deleted, even on partial failure
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ['catalogs'] });
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
    onSuccess: exitMode,
    onError: () => { setConfirmOpen(false); },
  });

  if (catsLoading || coursesLoading) return <div className="p-8 text-tx-3">Загрузка...</div>;

  const catalogSlugs = new Set((catalogs ?? []).flatMap(c => c.courses.map(x => x.slug)));
  const standalone = (courses ?? []).filter(c => !catalogSlugs.has(c.slug));

  const hasCatalogs = (catalogs?.length ?? 0) > 0;
  const hasStandalone = standalone.length > 0;
  const hasAny = hasCatalogs || hasStandalone;

  const cardCls = (selectedNow: boolean) =>
    clsx(
      'relative flex flex-col h-full bg-bg-2 border rounded-xl p-5 transition-all',
      editMode
        ? clsx('cursor-pointer select-none', selectedNow ? 'border-err shadow-md' : 'border-bdr hover:border-bdr-e')
        : 'border-bdr hover:border-bdr-e hover:shadow-md hover:-translate-y-px group',
    );

  return (
    <div className="overflow-auto h-full">
      <div className="max-w-5xl mx-auto px-6 py-12">
        <h1 className="text-2xl font-semibold text-tx-1 mb-8">Курсы</h1>

        {(hasCatalogs || editMode) && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 auto-rows-fr gap-4 mb-8">
            <AnimatePresence mode="popLayout">
            {(catalogs ?? []).map((catalog, i) => {
              const key = catKey(catalog.slug);
              const sel = selected.has(key);
              const content = (
                <>
                  {editMode && <SelectMark on={sel} />}
                  <div className="flex items-start gap-3 mb-2">
                    <span className="text-accent mt-0.5 shrink-0">
                      <FolderIcon />
                    </span>
                    <h2 className="text-tx-1 font-medium text-sm leading-snug">{catalog.title}</h2>
                  </div>
                  {catalog.description && (
                    <p className="text-tx-3 text-xs line-clamp-2 mb-3">{catalog.description}</p>
                  )}
                  <p className="text-tx-3 text-xs mt-auto">{catalog.courses.length} курс{pluralRu(catalog.courses.length)}</p>
                </>
              );
              return (
                <motion.div
                  key={catalog.slug}
                  className="h-full"
                  layout="position"
                  initial={{ opacity: 0, y: 16 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={dustExit(i)}
                  transition={{ delay: i * 0.05, duration: 0.25 }}
                >
                  {editMode ? (
                    <div className={cardCls(sel)} onClick={() => toggle(key)}>{content}</div>
                  ) : (
                    <Link to={`/catalogs/${catalog.slug}`} className={cardCls(false)}>{content}</Link>
                  )}
                </motion.div>
              );
            })}
            </AnimatePresence>
            <AnimatePresence mode="popLayout">
              {editMode && (
                <motion.button
                  key="create-group"
                  layout
                  onClick={() => setCreating(true)}
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.9 }}
                  transition={{ duration: 0.2 }}
                  className="flex flex-col items-center justify-center gap-2 min-h-[7rem] rounded-xl border border-dashed border-brand/40 bg-brand/5 text-brand hover:bg-brand/10 hover:border-brand/60 transition-colors"
                >
                  <PlusIcon />
                  <span className="text-sm font-medium">Создать группу</span>
                </motion.button>
              )}
            </AnimatePresence>
          </div>
        )}

        {hasStandalone && (
          <motion.div layout="position" transition={{ layout: { duration: 0.25, ease: 'easeInOut' } }}>
            {hasCatalogs && <h2 className="text-sm font-medium text-tx-3 mb-4">Отдельные курсы</h2>}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 auto-rows-fr gap-4">
              <AnimatePresence mode="popLayout">
              {standalone.map((course, i) => {
                const key = crsKey(course.slug);
                const sel = selected.has(key);
                const content = (
                  <>
                    {editMode && <SelectMark on={sel} />}
                    <h2 className="text-tx-1 font-medium text-sm leading-snug mb-2">{course.title}</h2>
                    {course.description && (
                      <p className="text-tx-3 text-xs line-clamp-2">{course.description}</p>
                    )}
                    <p className="text-tx-3 text-xs mt-auto pt-3">
                      {course.theory_count} {pluralTheory(course.theory_count)} · {course.task_count} {pluralTask(course.task_count)}
                    </p>
                  </>
                );
                return (
                  <motion.div
                    key={course.slug}
                    className="h-full"
                    layout="position"
                    initial={{ opacity: 0, y: 16 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={dustExit(i)}
                    transition={{ delay: i * 0.05, duration: 0.25 }}
                  >
                    {editMode ? (
                      <div className={cardCls(sel)} onClick={() => toggle(key)}>{content}</div>
                    ) : (
                      <Link to={`/courses/${course.slug}`} className={cardCls(false)}>{content}</Link>
                    )}
                  </motion.div>
                );
              })}
              </AnimatePresence>
            </div>
          </motion.div>
        )}

        {!hasAny && (
          <p className="text-tx-3 text-sm">Нет загруженных курсов. Добавь курсы в настройках.</p>
        )}
      </div>

      {/* exit (×) — top right, below header */}
      <AnimatePresence>
        {editMode && (
          <motion.button
            key="exit"
            onClick={exitMode}
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.8 }}
            className="fixed top-[68px] right-6 z-40 w-9 h-9 rounded-full bg-bg-2 border border-bdr text-tx-2 hover:text-tx-1 hover:border-bdr-e flex items-center justify-center shadow-md text-lg leading-none transition-colors"
            aria-label="Выйти из режима редактирования"
          >
            ×
          </motion.button>
        )}
      </AnimatePresence>

      {/* bottom-right action */}
      {hasAny && (
        <div className="fixed bottom-6 right-6 z-40 flex flex-col items-end gap-2">
          {editMode && delMut.isError && (
            <span className="px-3 py-1.5 rounded-lg bg-bg-2 border border-err text-err text-xs shadow-md">
              {(delMut.error as Error).message}
            </span>
          )}
          <AnimatePresence mode="wait">
            {editMode ? (
              <motion.button
                key="delete"
                onClick={() => { setPurge(false); setConfirmOpen(true); }}
                disabled={selected.size === 0}
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 12 }}
                className={clsx(
                  'px-4 h-11 rounded-full text-sm font-medium shadow-lg flex items-center gap-2 transition-colors',
                  selected.size === 0
                    ? 'bg-bg-2 border border-bdr text-tx-3 cursor-not-allowed'
                    : 'bg-err text-white hover:opacity-90',
                )}
              >
                <TrashIcon />
                {`Удалить${selected.size ? ` (${selected.size})` : ''}`}
              </motion.button>
            ) : (
              <motion.button
                key="enter"
                onClick={() => setEditMode(true)}
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 12 }}
                className="w-11 h-11 rounded-full bg-bg-2 border border-bdr text-tx-3 hover:text-brand hover:border-brand/50 shadow-lg flex items-center justify-center transition-colors"
                aria-label="Режим редактирования"
              >
                <PencilIcon />
              </motion.button>
            )}
          </AnimatePresence>
        </div>
      )}

      <AnimatePresence>
        {creating && (
          <CreateGroupDialog
            pending={createMut.isPending}
            error={createMut.error as Error | null}
            onClose={() => { setCreating(false); createMut.reset(); }}
            onSubmit={(title, description) => createMut.mutate({ title, description })}
          />
        )}
        {confirmOpen && (
          <ConfirmDeleteDialog
            catalogCount={[...selected].filter(k => k.startsWith('cat:')).length}
            courseCount={[...selected].filter(k => k.startsWith('crs:')).length}
            purge={purge}
            onTogglePurge={() => setPurge(p => !p)}
            pending={delMut.isPending}
            onClose={() => { setConfirmOpen(false); delMut.reset(); }}
            onConfirm={() => delMut.mutate(purge)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

function CreateGroupDialog({ pending, error, onClose, onSubmit }: {
  pending: boolean;
  error: Error | null;
  onClose: () => void;
  onSubmit: (title: string, description: string) => void;
}) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const canSubmit = title.trim().length > 0 && !pending;

  return (
    <motion.div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className="w-full max-w-md rounded-xl bg-bg-2 border border-bdr p-6 shadow-xl"
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 8 }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-tx-1 mb-4">Новая группа</h2>
        <input
          autoFocus
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Название"
          className="w-full mb-3 px-3 py-2 rounded-lg bg-bg-1 border border-bdr text-tx-1 text-sm placeholder:text-tx-3 focus:border-brand focus:outline-none"
        />
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Описание (необязательно)"
          rows={3}
          className="w-full mb-1 px-3 py-2 rounded-lg bg-bg-1 border border-bdr text-tx-1 text-sm placeholder:text-tx-3 focus:border-brand focus:outline-none resize-none"
        />
        {error && <p className="text-err text-xs mb-2">{error.message}</p>}
        <div className="flex justify-end gap-2 mt-4">
          <button
            onClick={onClose}
            className="px-4 h-9 rounded-lg text-tx-2 text-sm hover:text-tx-1 transition-colors"
          >
            Отмена
          </button>
          <button
            onClick={() => onSubmit(title.trim(), description.trim())}
            disabled={!canSubmit}
            className="px-4 h-9 rounded-lg bg-brand text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity"
          >
            {pending ? 'Создание...' : 'Создать'}
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

function ConfirmDeleteDialog({ catalogCount, courseCount, purge, onTogglePurge, pending, onClose, onConfirm }: {
  catalogCount: number;
  courseCount: number;
  purge: boolean;
  onTogglePurge: () => void;
  pending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const parts: string[] = [];
  if (catalogCount) parts.push(`${catalogCount} ${pluralGroup(catalogCount)}`);
  if (courseCount) parts.push(`${courseCount} курс${pluralRu(courseCount)}`);

  return (
    <motion.div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className="w-full max-w-md rounded-xl bg-bg-2 border border-bdr p-6 shadow-xl"
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 8 }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-tx-1 mb-2">Удалить {parts.join(' и ')}?</h2>
        <p className="text-tx-3 text-sm mb-4">Действие необратимо.</p>

        {catalogCount > 0 && (
          <button
            onClick={onTogglePurge}
            className="w-full flex items-start gap-3 p-3 mb-4 rounded-lg border border-bdr text-left hover:bg-bg-3 transition-colors"
          >
            <span className={clsx(
              'mt-0.5 shrink-0 w-5 h-5 rounded border flex items-center justify-center transition-colors',
              purge ? 'bg-err border-err text-white' : 'border-bdr text-transparent',
            )}>
              <CheckIcon />
            </span>
            <span className="min-w-0">
              <span className="block text-tx-1 text-sm">Удалить курсы внутри групп</span>
              <span className="block text-tx-3 text-xs mt-0.5">
                {purge
                  ? 'Все курсы из выбранных групп будут стёрты с диска безвозвратно.'
                  : 'Курсы из групп сохранятся как отдельные курсы.'}
              </span>
            </span>
          </button>
        )}

        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-4 h-9 rounded-lg text-tx-2 text-sm hover:text-tx-1 transition-colors"
          >
            Отмена
          </button>
          <button
            onClick={onConfirm}
            disabled={pending}
            className="px-4 h-9 rounded-lg bg-err text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity"
          >
            {pending ? 'Удаление...' : 'Удалить'}
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

function pluralRu(n: number): string {
  if (n % 10 === 1 && n % 100 !== 11) return '';
  if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'а';
  return 'ов';
}

function pluralTheory(n: number): string {
  if (n % 10 === 1 && n % 100 !== 11) return 'теория';
  if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'теории';
  return 'теорий';
}

function pluralTask(n: number): string {
  if (n % 10 === 1 && n % 100 !== 11) return 'задача';
  if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'задачи';
  return 'задач';
}

// accusative form of "группа" for "удалить N …"
function pluralGroup(n: number): string {
  if (n % 10 === 1 && n % 100 !== 11) return 'группу';
  if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'группы';
  return 'групп';
}
