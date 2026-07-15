import { useState, useRef } from 'react';
import { motion } from 'framer-motion';
import { useParams, useNavigate, Outlet } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { ProgressBar } from '../components/ui/ProgressBar';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import type { TrackItem } from '../api/types';
import { buildTree, initialOpen, type TreeNode, type NavTarget } from '../lib/buildTree';

interface SidebarProps {
  tracks: TrackItem[];
  done: Record<string, boolean>;
  activeTaskSlug?: string;
  activeUnitSlug?: string;
  onTask: (nav: NavTarget) => void;
  onTheory: (nav: NavTarget) => void;
  onResetProgress: () => void;
}

interface RowProps {
  node: TreeNode;
  depth: number;
  open: Record<string, boolean>;
  toggle: (id: string) => void;
  activeTaskSlug?: string;
  activeUnitSlug?: string;
  onTask: (nav: NavTarget) => void;
  onTheory: (nav: NavTarget) => void;
}

function TreeRow({ node, depth, open, toggle, activeTaskSlug, activeUnitSlug, onTask, onTheory }: RowProps) {
  const indent = depth > 0 ? 'ml-3' : '';

  if (node.kind === 'group') {
    const isOpen = open[node.id];
    const topLevel = depth === 0;
    return (
      <div className={indent}>
        <button
          onClick={() => toggle(node.id)}
          className={clsx(
            'w-full flex items-center gap-2 rounded hover:bg-bg-4 transition-colors text-left',
            topLevel ? 'px-2 py-1.5' : 'px-2 py-1',
          )}
        >
          <span className={clsx('text-tx-3 text-xs transition-transform', isOpen && 'rotate-90')}>▶</span>
          <span
            className={clsx('flex-1 truncate', topLevel ? 'text-sm font-medium text-tx-1' : 'text-xs text-tx-2')}
          >
            {node.title}
          </span>
          {node.total > 0 && <span className="text-xs text-tx-3 shrink-0">{node.done}/{node.total}</span>}
        </button>
        {isOpen && (
          <div className="mt-0.5 space-y-0.5">
            {node.children.map((c) => (
              <TreeRow
                key={c.id}
                node={c}
                depth={depth + 1}
                open={open}
                toggle={toggle}
                activeTaskSlug={activeTaskSlug}
                activeUnitSlug={activeUnitSlug}
                onTask={onTask}
                onTheory={onTheory}
              />
            ))}
            {topLevel && node.total > 0 && <ProgressBar value={node.done} max={node.total} className="mx-2 mt-1 mb-2" />}
          </div>
        )}
      </div>
    );
  }

  const isTask = node.kind === 'task';
  const active = isTask ? activeTaskSlug === node.nav?.task : activeUnitSlug === node.nav?.unit;
  return (
    <div className={indent}>
      <button
        onClick={() => (isTask ? onTask(node.nav!) : onTheory(node.nav!))}
        className={clsx(
          'w-full flex items-center gap-2 px-2 py-1 rounded text-left transition-colors text-xs',
          active ? 'bg-brand-subtle text-brand' : 'text-tx-2 hover:bg-bg-4 hover:text-tx-1',
        )}
      >
        <span className={clsx('text-xs', node.doneFlag ? 'text-ok' : 'text-tx-3')}>
          {node.doneFlag ? '✓' : '·'}
        </span>
        <span className="truncate">{node.title}</span>
      </button>
    </div>
  );
}

function Sidebar({ tracks, done, activeTaskSlug, activeUnitSlug, onTask, onTheory, onResetProgress }: SidebarProps) {
  const tree = buildTree(tracks, done);
  const [open, setOpen] = useState<Record<string, boolean>>(() =>
    initialOpen(tree, activeTaskSlug, activeUnitSlug),
  );
  const toggle = (id: string) => setOpen((m) => ({ ...m, [id]: !m[id] }));
  const hasProgress = Object.keys(done).length > 0;

  return (
    <nav className="w-64 shrink-0 border-r border-bdr bg-bg-2 h-full flex flex-col">
      <div className="p-3 space-y-1 flex-1 overflow-y-auto">
        {tree.map((node) => (
          <TreeRow
            key={node.id}
            node={node}
            depth={0}
            open={open}
            toggle={toggle}
            activeTaskSlug={activeTaskSlug}
            activeUnitSlug={activeUnitSlug}
            onTask={onTask}
            onTheory={onTheory}
          />
        ))}
      </div>
      {hasProgress && (
        <div className="p-3 border-t border-bdr">
          <button
            onClick={onResetProgress}
            className="w-full px-2 py-1.5 rounded text-left text-xs text-tx-3 hover:text-err hover:bg-bg-4 transition-colors"
          >
            Сбросить прогресс
          </button>
        </div>
      )}
    </nav>
  );
}

export interface CoursePageContext {
  mainRef: React.RefObject<HTMLElement | null>;
}

export function CoursePage() {
  const { courseSlug } = useParams<{ courseSlug: string }>();
  const navigate = useNavigate();
  const mainRef = useRef<HTMLElement>(null);
  const qc = useQueryClient();
  const [resetOpen, setResetOpen] = useState(false);

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
  const { taskSlug, unitSlug } = useParams<{ taskSlug?: string; unitSlug?: string }>();

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
    <div className="flex h-full overflow-hidden">
      <motion.div
        className="shrink-0 h-full"
        initial={{ x: -28, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        exit={{ x: -28, opacity: 0 }}
        transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
      >
        <Sidebar
          tracks={course.tracks}
          done={done}
          activeTaskSlug={taskSlug}
          activeUnitSlug={unitSlug}
          onTask={handleTask}
          onTheory={handleTheory}
          onResetProgress={() => setResetOpen(true)}
        />
      </motion.div>
      <ConfirmDialog
        open={resetOpen}
        title="Сбросить прогресс?"
        message="Все отметки о выполненных задачах этого курса будут сняты. Сабмиты останутся."
        confirmLabel={resetMut.isPending ? 'Сброс...' : 'Сбросить'}
        onConfirm={() => resetMut.mutate()}
        onCancel={() => setResetOpen(false)}
      />
      <motion.main
        ref={mainRef}
        className="flex-1 overflow-auto"
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: 12, transition: { duration: 0.22, ease: [0.22, 1, 0.36, 1] } }}
        transition={{ duration: 0.32, ease: [0.22, 1, 0.36, 1], delay: 0.06 }}
      >
        <Outlet context={{ mainRef } satisfies CoursePageContext} />
      </motion.main>
    </div>
  );
}
