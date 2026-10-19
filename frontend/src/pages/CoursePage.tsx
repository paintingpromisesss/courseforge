import { useState, useRef, useEffect, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useParams, useNavigate, useLocation, useOutlet } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import type { CourseDetail } from '../api/types';
import type { NavTarget } from '../lib/buildTree';
import { DockProvider, type CenterTabsConfig } from '../layout/DockContext';
import { DockLayout } from '../layout/DockLayout';
import { CourseSidebar } from '../layout/CourseSidebar';
import { GitDockPanel } from '../git/GitDockPanel';
import type { CenterTab } from '../layout/dock';

export interface CoursePageContext {
  mainRef: React.RefObject<HTMLElement | null>;
  course?: CourseDetail;
}

const taskTransitionVariants = {
  initial: (fromEmpty: boolean) =>
    fromEmpty
      ? { opacity: 0, y: 16 }
      : { opacity: 0 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.18,
      ease: 'easeOut' as const,
    },
  },
  exit: (fromEmpty: boolean) =>
    fromEmpty
      ? { opacity: 0, y: -10, transition: { duration: 0.14 } }
      : { opacity: 0, transition: { duration: 0.14 } },
};

export function CoursePage() {
  const { courseSlug, trackSlug, topicSlug, unitSlug, taskSlug } = useParams<{
    courseSlug: string;
    trackSlug?: string;
    topicSlug?: string;
    unitSlug?: string;
    taskSlug?: string;
  }>();
  const navigate = useNavigate();
  const location = useLocation();
  const mainRef = useRef<HTMLElement>(null);
  const qc = useQueryClient();
  const [resetOpen, setResetOpen] = useState(false);

  // Group key unites tasks within the same unit (common theory).
  // Navigating between tasks in the same unit does NOT unmount TaskPage,
  // allowing theory to stay completely untouched and avoid auto-switching.
  const currentGroupKey = unitSlug
    ? `group:${trackSlug ?? ''}/${topicSlug ?? ''}/${unitSlug}${taskSlug ? ':task' : ':theory'}`
    : 'empty';
  const prevGroupKeyRef = useRef<string>('empty');
  const wasEmpty = prevGroupKeyRef.current === 'empty';

  useEffect(() => {
    prevGroupKeyRef.current = currentGroupKey;
  }, [currentGroupKey]);

  const { data: course, isLoading } = useQuery({
    queryKey: ['course', courseSlug],
    queryFn: () => api.getCourse(courseSlug!),
    enabled: !!courseSlug,
  });

  const { data: progress } = useQuery({
    queryKey: ['progress', courseSlug],
    queryFn: () => api.getProgress(courseSlug!),
    enabled: !!courseSlug,
  });

  const done = progress?.completed_tasks ?? {};

  const unit = useMemo(
    () => course?.tracks.flatMap((t) => t.topics).flatMap((p) => p.units).find((u) => u.slug === unitSlug),
    [course, unitSlug],
  );
  const activeTask = useMemo(
    () => unit?.tasks.find((t) => t.slug === taskSlug),
    [unit, taskSlug],
  );
  const firstTask = unit?.tasks[0];

  // Remember the last visited spot for the "Продолжить обучение" banner on the home page.
  useEffect(() => {
    if (!course || !courseSlug) return;
    // During the exit animation this component is still mounted while
    // useLocation already points at the next route — don't record foreign paths.
    if (!location.pathname.startsWith(`/courses/${courseSlug}`)) return;
    let label: string | undefined;
    for (const tr of course.tracks) {
      for (const tp of tr.topics) {
        for (const u of tp.units) {
          if (unitSlug && u.slug === unitSlug && !taskSlug) label = u.title;
          for (const t of u.tasks) if (taskSlug && t.slug === taskSlug) label = t.title;
        }
      }
    }
    localStorage.setItem(
      'cf:last-visit',
      JSON.stringify({ slug: courseSlug, title: course.title, label, path: location.pathname }),
    );
  }, [course, courseSlug, taskSlug, unitSlug, location.pathname]);

  const resetMut = useMutation({
    mutationFn: () => api.resetProgress(courseSlug!),
    onSuccess: () => {
      setResetOpen(false);
      qc.invalidateQueries({ queryKey: ['progress', courseSlug] });
      // course cards show done_count — refresh them too
      qc.invalidateQueries({ queryKey: ['courses'] });
      qc.invalidateQueries({ queryKey: ['catalogs'] });
    },
  });

  const outlet = useOutlet({ mainRef, course } satisfies CoursePageContext);

  // Center switcher config for the action bar (unit/task pages only).
  const centerTabs: CenterTabsConfig | null = useMemo(() => {
    if (!courseSlug || !unit) return null;
    const openTask = (view: CenterTab) => {
      if (!firstTask) return;
      const url =
        `/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/tasks/${firstTask.slug}`;
      navigate(url, { state: { taskView: view } });
    };
    return {
      unitHasTheory: !!unit.has_theory,
      // видео-разбор существует только в контексте текущей задачи
      hasTaskVideo: !!activeTask && !!(
        activeTask.video_url || activeTask.editorial_url || (activeTask.video_sources && activeTask.video_sources.length > 0)
      ),
      canOpenTask: !!firstTask,
      onOpenTask: openTask,
    };
  }, [courseSlug, unit, firstTask, activeTask, trackSlug, topicSlug, unitSlug, navigate]);

  if (isLoading) return <div className="p-8 text-tx-3">Загрузка...</div>;
  if (!course) return <div className="p-8 text-err">Курс не найден</div>;

  const handleTask = (nav: NavTarget) => {
    navigate(
      `/courses/${courseSlug}/tracks/${nav.track}/topics/${nav.topic}/units/${nav.unit}/tasks/${nav.task}`,
    );
  };

  const handleTheory = (nav: NavTarget) => {
    navigate(
      `/courses/${courseSlug}/tracks/${nav.track}/topics/${nav.topic}/units/${nav.unit}/theory`,
    );
  };

  return (
    <DockProvider>
      <DockLayout
        tree={
          <CourseSidebar
            tracks={course.tracks}
            done={done}
            activeTaskSlug={taskSlug}
            activeUnitSlug={unitSlug}
            onTask={handleTask}
            onTheory={handleTheory}
            onResetProgress={() => setResetOpen(true)}
          />
        }
        git={courseSlug ? <GitDockPanel courseSlug={courseSlug} /> : null}
        centerTabs={centerTabs}
      >
        <ConfirmDialog
          open={resetOpen}
          title="Сбросить прогресс?"
          message="Все отметки о выполненных задачах этого курса будут сняты. Сабмиты останутся."
          confirmLabel={resetMut.isPending ? 'Сброс...' : 'Сбросить'}
          onConfirm={() => resetMut.mutate()}
          onCancel={() => setResetOpen(false)}
        />
        <main
          ref={mainRef}
          className="h-full overflow-hidden relative"
        >
          <AnimatePresence mode="wait" custom={wasEmpty} initial={false}>
            <motion.div
              key={currentGroupKey}
              custom={wasEmpty}
              variants={taskTransitionVariants}
              initial="initial"
              animate="animate"
              exit="exit"
              className="h-full overflow-auto"
            >
              {outlet}
            </motion.div>
          </AnimatePresence>
        </main>
      </DockLayout>
    </DockProvider>
  );
}
