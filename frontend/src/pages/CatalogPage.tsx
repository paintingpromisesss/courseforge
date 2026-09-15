import { useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { api } from '../api/client';
import { CardDoneMark, CardProgressStrip, CourseDoneBadge } from '../components/ui/ProgressBar';

// "24 теории · 41 задача", zero parts omitted
function courseMeta(course: { theory_count: number; task_count: number }): string {
  return [
    course.theory_count > 0 && `${course.theory_count} ${pluralTheory(course.theory_count)}`,
    course.task_count > 0 && `${course.task_count} ${pluralTask(course.task_count)}`,
  ].filter(Boolean).join(' · ');
}
import type { CourseItem, CatalogItem } from '../api/types';

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

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
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

export function CatalogPage() {
  const { catalogSlug } = useParams<{ catalogSlug: string }>();
  const qc = useQueryClient();
  const [editMode, setEditMode] = useState(false);
  const [picking, setPicking] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [purge, setPurge] = useState(false);
  const [titleDraft, setTitleDraft] = useState('');
  const [descDraft, setDescDraft] = useState('');

  const { data: catalogs, isLoading, error } = useQuery({
    queryKey: ['catalogs'],
    queryFn: api.listCatalogs,
  });

  const patchMut = useMutation({
    mutationFn: (body: { title?: string; description?: string; courses?: string[] }) =>
      api.patchCatalog(catalogSlug!, body),
    onSuccess: () => {
      setPicking(false);
      qc.invalidateQueries({ queryKey: ['catalogs'] });
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
  });

  // keep a live ref so the unmount cleanup flushes the latest drafts.
  // Declared before the early returns so hook order stays stable.
  const flushRef = useRef<() => void>(() => {});
  useEffect(() => () => flushRef.current(), []);

  const delMut = useMutation({
    mutationFn: async (purgeDisk: boolean) => {
      if (purgeDisk) {
        const results = await Promise.allSettled([...selected].map(s => api.deleteCourse(s)));
        const failed = results.filter(r => r.status === 'rejected').length;
        if (failed) throw new Error(`Не удалось удалить: ${failed} из ${results.length}`);
      } else {
        await api.patchCatalog(catalogSlug!, { courses: memberSlugsRef.current.filter(s => !selected.has(s)) });
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ['catalogs'] });
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
    onSuccess: () => exitEditRef.current(),
    onError: () => setConfirmOpen(false),
  });

  // live refs so delMut (declared above the early returns) can reach values
  // computed after them without referencing TDZ'd consts
  const memberSlugsRef = useRef<string[]>([]);
  const exitEditRef = useRef<() => void>(() => {});

  if (isLoading) return <div className="p-8 text-tx-3">Загрузка...</div>;
  if (error) return <div className="p-8 text-err">Ошибка загрузки</div>;

  const catalog = catalogs?.find(c => c.slug === catalogSlug);
  if (!catalog) return <div className="p-8 text-err">Каталог не найден</div>;

  const memberSlugs = catalog.courses.map(c => c.slug);
  memberSlugsRef.current = memberSlugs;

  const toggle = (slug: string) =>
    setSelected(prev => {
      const next = new Set(prev);
      next.has(slug) ? next.delete(slug) : next.add(slug);
      return next;
    });

  // Persist pending title/description edits. Builds a minimal patch (only changed,
  // non-empty title) and fires it. Called on exit-from-edit-mode and on unmount.
  const flushMeta = () => {
    if (!editMode) return;
    const t = titleDraft.trim();
    const d = descDraft.trim();
    const body: { title?: string; description?: string } = {};
    if (t && t !== catalog.title) body.title = t;
    if (d !== (catalog.description ?? '')) body.description = d;
    if (!Object.keys(body).length) return;
    // optimistic: update the cached catalog so the new title/description shows
    // immediately on exit instead of flashing the old value until the refetch lands
    qc.setQueryData<CatalogItem[]>(['catalogs'], (old) =>
      old?.map(c => (c.slug === catalogSlug ? { ...c, ...body } : c)),
    );
    patchMut.mutate(body);
  };
  flushRef.current = flushMeta;

  const enterEdit = () => {
    setTitleDraft(catalog.title);
    setDescDraft(catalog.description ?? '');
    setEditMode(true);
  };
  const exitEdit = () => {
    flushMeta();
    setEditMode(false);
    setSelected(new Set());
    setConfirmOpen(false);
    setPurge(false);
  };
  exitEditRef.current = exitEdit;

  const cardCls = (sel: boolean) =>
    clsx(
      'relative flex flex-col h-full bg-bg-2 border rounded-xl p-5 transition-all overflow-hidden',
      editMode
        ? clsx('cursor-pointer select-none', sel ? 'border-err shadow-md' : 'border-bdr hover:border-bdr-e')
        : 'border-bdr hover:border-bdr-e hover:shadow-md hover:-translate-y-px group',
    );

  return (
    <div className="overflow-auto h-full">
      <div className="max-w-5xl mx-auto px-6 py-12">
        <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4 mb-8">
          <div className="flex-1 min-w-0">
            {editMode ? (
              <>
                <input
                  value={titleDraft}
                  onChange={(e) => setTitleDraft(e.target.value)}
                  placeholder="Название группы"
                  className="block w-full p-0 m-0 bg-transparent border-b border-bdr focus:border-brand pb-1 text-2xl font-semibold text-tx-1 placeholder:text-tx-3 focus:outline-none"
                />
                <textarea
                  value={descDraft}
                  onChange={(e) => setDescDraft(e.target.value)}
                  placeholder="Описание группы (необязательно)"
                  rows={2}
                  className="block w-full p-0 m-0 mt-2 bg-transparent border-b border-bdr-s focus:border-brand pb-1 text-sm text-tx-2 placeholder:text-tx-3 focus:outline-none resize-none"
                />
              </>
            ) : (
              <>
                <h1 className="text-2xl font-semibold text-tx-1">{catalog.title}</h1>
                {catalog.description && (
                  <p className="text-sm mt-1 text-tx-3">{catalog.description}</p>
                )}
              </>
            )}
          </div>

          <div className="flex items-center gap-2 shrink-0">
            {editMode ? (
              <>
                {delMut.isError && (
                  <span className="px-2.5 py-1 rounded-lg bg-err/10 border border-err/30 text-err text-xs">
                    {(delMut.error as Error).message}
                  </span>
                )}
                <button
                  onClick={() => { setPurge(false); setConfirmOpen(true); }}
                  disabled={selected.size === 0}
                  className={clsx(
                    'px-3.5 h-9 rounded-lg text-xs font-medium flex items-center gap-1.5 transition-all',
                    selected.size === 0
                      ? 'bg-bg-2 border border-bdr text-tx-3 cursor-not-allowed opacity-60'
                      : 'bg-err text-white hover:opacity-90 shadow-sm',
                  )}
                >
                  <TrashIcon />
                  {`Удалить${selected.size ? ` (${selected.size})` : ''}`}
                </button>
                <button
                  onClick={exitEdit}
                  className="px-3.5 h-9 rounded-lg border border-brand bg-brand text-white text-xs font-medium hover:bg-brand-hover transition-colors cursor-pointer"
                >
                  Готово
                </button>
              </>
            ) : (
              <>
                <button
                  onClick={() => setPicking(true)}
                  className="px-3.5 h-9 rounded-lg border border-brand/40 bg-brand/10 hover:bg-brand/20 text-brand text-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer"
                >
                  <PlusIcon />
                  <span>Добавить курсы</span>
                </button>
                <button
                  onClick={enterEdit}
                  className="px-3 h-9 rounded-lg border border-bdr bg-bg-2 hover:bg-bg-3 text-tx-3 hover:text-tx-1 text-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer"
                  title="Редактировать группу и список курсов"
                >
                  <PencilIcon />
                  <span>Редактировать</span>
                </button>
              </>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 auto-rows-fr gap-4">
          {catalog.courses.map((course, i) => {
            const sel = selected.has(course.slug);
            const card = (
              <>
                {editMode ? <SelectMark on={sel} /> : <CourseDoneBadge course={course} />}
                <h2 className="text-tx-1 font-medium text-sm leading-snug mb-2">{course.title}</h2>
                {course.description && (
                  <p className="text-tx-3 text-xs line-clamp-2">{course.description}</p>
                )}
                <p className="text-tx-3 text-xs mt-auto pt-3">
                  {courseMeta(course)}
                  <CardDoneMark course={course} />
                </p>
                <CardProgressStrip
                  value={course.done_count + course.theory_done_count}
                  max={course.task_count + course.theory_count}
                />
              </>
            );
            return (
              <motion.div
                key={course.slug}
                className="h-full"
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.05, duration: 0.25 }}
              >
                {editMode ? (
                  <div className={cardCls(sel)} onClick={() => toggle(course.slug)}>{card}</div>
                ) : (
                  <Link to={`/courses/${course.slug}`} className={cardCls(false)}>{card}</Link>
                )}
              </motion.div>
            );
          })}
          <AnimatePresence>
            {editMode && (
              <motion.button
                key="add-courses"
                onClick={() => setPicking(true)}
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                transition={{ duration: 0.2 }}
                className="flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-brand/40 bg-brand/5 text-brand hover:bg-brand/10 hover:border-brand/60 transition-colors"
              >
                <PlusIcon />
                <span className="text-sm font-medium">Добавить курсы</span>
              </motion.button>
            )}
          </AnimatePresence>
        </div>
      </div>



      <AnimatePresence>
        {picking && (
          <PickCoursesDialog
            memberSlugs={memberSlugs}
            pending={patchMut.isPending}
            error={patchMut.error as Error | null}
            onClose={() => { setPicking(false); patchMut.reset(); }}
            onSubmit={(slugs) => patchMut.mutate({ courses: slugs })}
          />
        )}
        {confirmOpen && (
          <ConfirmDeleteDialog
            count={selected.size}
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

function ConfirmDeleteDialog({ count, purge, onTogglePurge, pending, onClose, onConfirm }: {
  count: number;
  purge: boolean;
  onTogglePurge: () => void;
  pending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
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
        <h2 className="text-lg font-semibold text-tx-1 mb-2">Удалить {count} курс{pluralRu(count)}?</h2>
        <p className="text-tx-3 text-sm mb-4">
          {purge ? 'Действие необратимо.' : 'Курсы будут убраны из группы и останутся как отдельные.'}
        </p>

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
            <span className="block text-tx-1 text-sm">Удалить курсы с диска</span>
            <span className="block text-tx-3 text-xs mt-0.5">
              {purge
                ? 'Выбранные курсы будут стёрты с диска безвозвратно.'
                : 'Курсы сохранятся как отдельные курсы.'}
            </span>
          </span>
        </button>

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

function PickCoursesDialog({ memberSlugs, pending, error, onClose, onSubmit }: {
  memberSlugs: string[];
  pending: boolean;
  error: Error | null;
  onClose: () => void;
  onSubmit: (slugs: string[]) => void;
}) {
  const { data: courses, isLoading } = useQuery({
    queryKey: ['courses'],
    queryFn: api.listCourses,
  });
  const [selected, setSelected] = useState<Set<string>>(new Set(memberSlugs));

  const toggle = (slug: string) =>
    setSelected(prev => {
      const next = new Set(prev);
      next.has(slug) ? next.delete(slug) : next.add(slug);
      return next;
    });

  return (
    <motion.div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className="w-full max-w-md rounded-xl bg-bg-2 border border-bdr shadow-xl flex flex-col max-h-[80vh]"
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 8 }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-tx-1 px-6 pt-6 pb-4">Курсы в группе</h2>
        <div className="flex-1 overflow-auto px-3">
          {isLoading ? (
            <p className="text-tx-3 text-sm px-3 pb-4">Загрузка...</p>
          ) : (courses ?? []).length === 0 ? (
            <p className="text-tx-3 text-sm px-3 pb-4">Нет импортированных курсов.</p>
          ) : (
            (courses ?? []).map((c: CourseItem) => {
              const on = selected.has(c.slug);
              return (
                <button
                  key={c.slug}
                  onClick={() => toggle(c.slug)}
                  className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left hover:bg-bg-3 transition-colors"
                >
                  <span className={clsx(
                    'shrink-0 w-5 h-5 rounded border flex items-center justify-center transition-colors',
                    on ? 'bg-brand border-brand text-white' : 'border-bdr text-transparent',
                  )}>
                    <CheckIcon />
                  </span>
                  <span className="min-w-0">
                    <span className="block text-tx-1 text-sm truncate">{c.title}</span>
                    {c.description && (
                      <span className="block text-tx-3 text-xs truncate">{c.description}</span>
                    )}
                  </span>
                </button>
              );
            })
          )}
        </div>
        {error && <p className="text-err text-xs px-6 pt-2">{error.message}</p>}
        <div className="flex justify-end gap-2 px-6 py-4 border-t border-bdr">
          <button
            onClick={onClose}
            className="px-4 h-9 rounded-lg text-tx-2 text-sm hover:text-tx-1 transition-colors"
          >
            Отмена
          </button>
          <button
            onClick={() => onSubmit([...selected])}
            disabled={pending}
            className="px-4 h-9 rounded-lg bg-brand text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity"
          >
            {pending ? 'Сохранение...' : 'Изменить'}
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}
