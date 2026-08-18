import { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useParams, useNavigate, useLocation, Outlet } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { ProgressBar } from '../components/ui/ProgressBar';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import type { TrackItem } from '../api/types';
import { buildTree, initialOpen, type TreeNode, type NavTarget } from '../lib/buildTree';

function TreeRow({
  node,
  depth = 0,
  open,
  toggle,
  activeTaskSlug,
  activeUnitSlug,
  onTask,
  onTheory,
}: {
  node: TreeNode;
  depth?: number;
  open: Record<string, boolean>;
  toggle: (id: string) => void;
  activeTaskSlug?: string;
  activeUnitSlug?: string;
  onTask: (nav: NavTarget) => void;
  onTheory: (nav: NavTarget) => void;
}) {
  const isOpen = !!open[node.id];
  const isComplete = node.total > 0 && node.done === node.total;
  const isStarted = node.done > 0 && !isComplete;

  if (node.kind === 'group') {
    // Depth 0: Top-level Track / Week (Collapsible)
    if (depth === 0) {
      return (
        <div className="w-full">
          <button
            onClick={() => toggle(node.id)}
            className="w-full flex items-center gap-1.5 px-1.5 py-1.5 rounded-md hover:bg-bg-4 transition-colors text-left select-none group"
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 16 16"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className={clsx(
                'text-tx-3 group-hover:text-tx-2 transition-transform shrink-0',
                isOpen ? 'rotate-90' : 'rotate-0',
              )}
            >
              <polyline points="6 4 10 8 6 12" />
            </svg>

            <span className="flex-1 truncate text-sm font-semibold text-tx-1">
              {node.title}
            </span>

            {node.total > 0 && (
              <span
                className={clsx(
                  'text-xs font-semibold px-2 py-0.5 rounded-full shrink-0 flex items-center gap-1',
                  isComplete
                    ? 'bg-ok/15 text-ok'
                    : isStarted
                    ? 'bg-brand/15 text-brand'
                    : 'text-tx-3 bg-bg-4',
                )}
              >
                {isComplete ? `✓ ${node.done}/${node.total}` : `${node.done}/${node.total}`}
              </span>
            )}
          </button>

          <AnimatePresence initial={false}>
            {isOpen && (
              <motion.div
                key="track-content"
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.15, ease: 'easeInOut' }}
                style={{ overflow: 'hidden' }}
              >
                <div className="ml-2 space-y-1.5 pt-0.5 pb-1">
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
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      );
    }

    // Depth 1: Topic Section (Category with clean spacing and subtle indicator)
    if (depth === 1) {
      return (
        <div className="space-y-0.5 pt-1.5 first:pt-0">
          {/* Topic Header */}
          <div className="px-1.5 py-0.5 flex items-center justify-between gap-2 select-none group/topic">
            <div className="flex items-center gap-1.5 min-w-0 flex-1">
              <span className="w-1 h-3 rounded-full bg-brand/50 shrink-0" />
              <span className="text-xs font-semibold text-tx-1 truncate">
                {node.title}
              </span>
            </div>

            {node.total > 0 && (
              <span
                className={clsx(
                  'text-[11px] font-medium px-1.5 py-0.5 rounded-full shrink-0 flex items-center gap-0.5',
                  isComplete
                    ? 'bg-ok/15 text-ok'
                    : isStarted
                    ? 'bg-brand/15 text-brand'
                    : 'text-tx-3 bg-bg-4',
                )}
              >
                {isComplete ? `✓ ${node.done}/${node.total}` : `${node.done}/${node.total}`}
              </span>
            )}
          </div>

          {/* Topic Children with very subtle guide line */}
          <div className="ml-2 pl-2 space-y-0.5 border-l border-white/5">
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
          </div>
        </div>
      );
    }

    // Depth >= 2: Nested Sub-topic (e.g. sub-unit inside a topic)
    return (
      <div className="space-y-1 pt-1.5 first:pt-0">
        <div className="px-1.5 py-0.5 flex items-center justify-between text-tx-3 select-none">
          <div className="flex items-center gap-1.5 min-w-0 flex-1">
            <span className="text-tx-3 text-[10px] font-mono shrink-0">↳</span>
            <span className="text-xs font-medium text-tx-2 truncate">{node.title}</span>
          </div>
          {node.total > 0 && (
            <span
              className={clsx(
                'text-[11px] font-medium px-1.5 py-0.5 rounded-full shrink-0 flex items-center gap-0.5',
                isComplete
                  ? 'bg-ok/15 text-ok'
                  : isStarted
                  ? 'bg-brand/15 text-brand'
                  : 'text-tx-3 bg-bg-4',
              )}
            >
              {isComplete ? `✓ ${node.done}/${node.total}` : `${node.done}/${node.total}`}
            </span>
          )}
        </div>
        <div className="ml-2 pl-2 space-y-0.5 border-l border-white/5">
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
        </div>
      </div>
    );
  }

  // Leaf: task or theory
  const isTask = node.kind === 'task';
  const active = isTask ? activeTaskSlug === node.nav?.task : activeUnitSlug === node.nav?.unit;

  return (
    <button
      onClick={() => (isTask ? onTask(node.nav!) : onTheory(node.nav!))}
      title={node.title}
      className={clsx(
        'w-full flex items-center gap-2 px-1.5 py-1 rounded text-left transition-colors text-xs group/item',
        active
          ? 'bg-brand text-white font-medium shadow-sm'
          : 'text-tx-2 hover:bg-bg-4 hover:text-tx-1',
      )}
    >
      <span
        className={clsx(
          'w-3.5 text-center text-xs font-bold shrink-0',
          node.doneFlag
            ? active ? 'text-white' : 'text-ok'
            : active ? 'text-white/60' : 'text-tx-3',
        )}
      >
        {node.doneFlag ? '✓' : '·'}
      </span>
      <span className="truncate flex-1">{node.title}</span>
    </button>
  );
}

function computeAutoFitWidth(tree: TreeNode[], open: Record<string, boolean>): number {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  const measure = (text: string, font: string) => {
    if (!ctx) return text.length * 8.5;
    ctx.font = font;
    return ctx.measureText(text).width;
  };

  let maxW = 270;

  const traverse = (node: TreeNode, depth: number) => {
    let rowWidth = 0;
    if (depth === 0) {
      // Track
      const titleW = measure(node.title, '600 14px Inter, system-ui, sans-serif');
      const badgeW = node.total > 0 ? 60 : 0;
      rowWidth = 16 + 16 + titleW + 8 + badgeW + 28;
    } else if (node.kind === 'group') {
      // Topic or Subgroup
      const indent = 8 + (depth - 1) * 16;
      const titleW = measure(node.title, '600 12px Inter, system-ui, sans-serif');
      const badgeW = node.total > 0 ? 55 : 0;
      rowWidth = 16 + indent + 16 + titleW + 8 + badgeW + 28;
    } else {
      // Task or Theory Leaf
      const indent = 8 + (depth - 1) * 16;
      const titleW = measure(node.title, '400 12px Inter, system-ui, sans-serif');
      rowWidth = 16 + indent + 22 + titleW + 36;
    }

    if (rowWidth > maxW) maxW = rowWidth;

    if (node.kind === 'group' && (depth === 0 ? open[node.id] : true)) {
      for (const child of node.children) {
        traverse(child, depth + 1);
      }
    }
  };

  for (const root of tree) {
    traverse(root, 0);
  }

  return Math.min(Math.max(Math.ceil(maxW), 260), 650);
}

function Sidebar({ title, tracks, done, activeTaskSlug, activeUnitSlug, onTask, onTheory, onResetProgress }: SidebarProps) {
  const tree = useMemo(() => buildTree(tracks, done), [tracks, done]);
  const treeRef = useRef<HTMLDivElement>(null);

  const [open, setOpen] = useState<Record<string, boolean>>(() => {
    const init: Record<string, boolean> = {};
    if (activeTaskSlug || activeUnitSlug) {
      for (const track of tree) {
        const containsActive = (n: TreeNode): boolean => {
          if (n.kind === 'task') return n.nav?.task === activeTaskSlug;
          if (n.kind === 'theory') return n.nav?.unit === activeUnitSlug;
          return n.children.some(containsActive);
        };
        if (containsActive(track)) {
          init[track.id] = true;
        }
      }
    }
    return init;
  });

  const [sidebarWidth, setSidebarWidth] = useState<number>(() => {
    const saved = localStorage.getItem('cf:sidebar-width');
    if (saved) {
      const parsed = parseInt(saved, 10);
      if (!isNaN(parsed) && parsed >= 200 && parsed <= 650) return parsed;
    }
    // Calculate initial auto-fit based on initial open items
    const initOpen: Record<string, boolean> = {};
    if (activeTaskSlug || activeUnitSlug) {
      for (const track of tree) {
        const containsActive = (n: TreeNode): boolean => {
          if (n.kind === 'task') return n.nav?.task === activeTaskSlug;
          if (n.kind === 'theory') return n.nav?.unit === activeUnitSlug;
          return n.children.some(containsActive);
        };
        if (containsActive(track)) initOpen[track.id] = true;
      }
    }
    return computeAutoFitWidth(tree, initOpen);
  });

  const [isDragging, setIsDragging] = useState(false);

  const measureRealDom = useCallback((): number => {
    if (!treeRef.current) return computeAutoFitWidth(tree, open);
    const container = treeRef.current;
    const containerRect = container.getBoundingClientRect();
    let maxNeeded = 270;

    const rows = container.querySelectorAll<HTMLElement>('button, .select-none');
    rows.forEach((row) => {
      const textSpan = row.querySelector<HTMLElement>('.truncate');
      if (!textSpan || !textSpan.textContent) return;
      const rowRect = row.getBoundingClientRect();
      const leftOffset = rowRect.left - containerRect.left;

      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (ctx) {
        const computed = window.getComputedStyle(textSpan);
        ctx.font = `${computed.fontWeight} ${computed.fontSize} ${computed.fontFamily}`;
        const naturalTextWidth = ctx.measureText(textSpan.textContent).width;
        const badge = row.querySelector<HTMLElement>('.rounded-full');
        const badgeW = badge ? badge.offsetWidth + 12 : 0;
        const iconW = 24;

        const totalNeeded = leftOffset + iconW + naturalTextWidth + badgeW + 36;
        if (totalNeeded > maxNeeded) {
          maxNeeded = totalNeeded;
        }
      }
    });

    return Math.min(Math.max(Math.ceil(maxNeeded), 260), 650);
  }, [tree, open]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setIsDragging(true);
    const startX = e.clientX;
    const startWidth = sidebarWidth;

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX;
      const newWidth = Math.min(Math.max(startWidth + delta, 200), 650);
      setSidebarWidth(newWidth);
      localStorage.setItem('cf:sidebar-width', String(newWidth));
    };

    const onMouseUp = () => {
      setIsDragging(false);
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }, [sidebarWidth]);

  const handleDoubleClick = () => {
    const autoW = measureRealDom();
    setSidebarWidth(autoW);
    localStorage.setItem('cf:sidebar-width', String(autoW));
  };

  // Auto-expand track when active task or unit changes
  useEffect(() => {
    if (!activeTaskSlug && !activeUnitSlug) return;
    setOpen((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const track of tree) {
        const containsActive = (n: TreeNode): boolean => {
          if (n.kind === 'task') return n.nav?.task === activeTaskSlug;
          if (n.kind === 'theory') return n.nav?.unit === activeUnitSlug;
          return n.children.some(containsActive);
        };
        if (containsActive(track) && !next[track.id]) {
          next[track.id] = true;
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [activeTaskSlug, activeUnitSlug, tree]);

  const toggle = (id: string) => setOpen((m) => ({ ...m, [id]: !m[id] }));

  // Expand all / Collapse all helper for top tracks
  const trackIds = useMemo(() => tree.filter((n) => n.kind === 'group').map((n) => n.id), [tree]);
  const allExpanded = trackIds.length > 0 && trackIds.every((id) => open[id]);

  const toggleAll = () => {
    const nextState = !allExpanded;
    const nextOpen: Record<string, boolean> = {};
    for (const id of trackIds) {
      nextOpen[id] = nextState;
    }
    setOpen(nextOpen);
  };

  const hasProgress = Object.keys(done).length > 0;
  const totalDone = tree.reduce((a, n) => a + n.done, 0);
  const total = tree.reduce((a, n) => a + n.total, 0);

  return (
    <div className="flex h-full shrink-0">
      <nav
        style={{ width: sidebarWidth }}
        className={clsx('shrink-0 bg-bg-2 h-full flex flex-col', isDragging && 'select-none')}
      >
        {/* Top Toolbar */}
        <div className="h-11 shrink-0 px-3 border-b border-bdr flex items-center justify-between gap-2 select-none">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-tx-3">
            <svg
              width="12"
              height="12"
              viewBox="0 0 12 12"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              className="shrink-0 text-tx-3 -translate-y-[0.5px]"
            >
              <line x1="1" y1="2" x2="11" y2="2" />
              <line x1="1" y1="6" x2="11" y2="6" />
              <line x1="1" y1="10" x2="11" y2="10" />
            </svg>
            <span>Содержание</span>
          </div>

          <button
            type="button"
            onClick={toggleAll}
            className="flex items-center gap-1.5 text-xs font-medium text-tx-2 hover:text-tx-1 bg-bg-3 hover:bg-bg-4 px-2.5 py-1 rounded-md active:scale-95 transition-all"
          >
            <svg
              width="13"
              height="13"
              viewBox="0 0 16 16"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="text-tx-3 shrink-0"
            >
              {allExpanded ? (
                <>
                  <polyline points="4 2 8 6 12 2" />
                  <polyline points="4 14 8 10 12 14" />
                </>
              ) : (
                <>
                  <polyline points="4 6 8 2 12 6" />
                  <polyline points="4 10 8 14 12 10" />
                </>
              )}
            </svg>
            <span>{allExpanded ? 'Свернуть' : 'Развернуть'}</span>
          </button>
        </div>

        {/* Tree Content */}
        <div ref={treeRef} className="px-2 py-2.5 space-y-1 flex-1 overflow-y-auto">
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

        {/* Bottom Progress & Actions Panel */}
        {total > 0 && (
          <div className="p-3 border-t border-bdr bg-bg-2 select-none space-y-2.5">
            <div className="flex items-center justify-between text-xs">
              <span className="text-tx-3 font-medium">Прогресс</span>
              <span className={clsx('font-semibold text-xs', totalDone === total ? 'text-ok' : 'text-tx-2')}>
                {totalDone === total ? `✓ ${totalDone}/${total}` : `${totalDone}/${total}`} ({Math.round((totalDone / total) * 100)}%)
              </span>
            </div>
            <ProgressBar value={totalDone} max={total} />

            {hasProgress && (
              <button
                onClick={onResetProgress}
                className="w-full mt-2 py-1.5 px-2.5 rounded-md text-xs font-medium text-tx-3 hover:text-err bg-bg-3 hover:bg-bg-4 transition-all flex items-center justify-center gap-1.5 active:scale-98"
              >
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 16 16"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="shrink-0"
                >
                  <path d="M2.5 2v4h4" />
                  <path d="M3.5 10a6 6 0 1 0 1.5-6.5L2.5 6" />
                </svg>
                <span>Сбросить прогресс</span>
              </button>
            )}
          </div>
        )}
      </nav>

      {/* Standalone Splitter between Sidebar and Main Content */}
      <div
        onMouseDown={handleMouseDown}
        onDoubleClick={handleDoubleClick}
        className={clsx(
          'w-px shrink-0 bg-bdr cursor-col-resize z-20 transition-colors relative',
          isDragging ? 'bg-brand shadow-[0_0_8px_rgba(124,58,237,0.5)]' : 'hover:bg-brand/60',
        )}
      >
        <div className="absolute inset-y-0 left-0 w-1.5 z-30" />
      </div>
    </div>
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
  const location = useLocation();

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
          title={course.title}
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
