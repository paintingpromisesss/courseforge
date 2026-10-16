import { useState, useMemo, useEffect, useCallback, memo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { ProgressBar } from '../components/ui/ProgressBar';
import { DifficultyBadge } from '../components/ui/DifficultyBadge';
import type { TrackItem } from '../api/types';
import { buildTree, type TreeNode, type NavTarget } from '../lib/buildTree';
import { useDock } from './DockContext';

const TreeRow = memo(function TreeRow({
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
          <div className="ml-2 pl-2 space-y-0.5 border-l border-bdr-s">
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
        <div className="ml-2 pl-2 space-y-0.5 border-l border-bdr-s">
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
      {isTask && node.difficulty ? (
        <DifficultyBadge difficulty={node.difficulty} size="sm" active={active} />
      ) : null}
    </button>
  );
});

interface CourseSidebarProps {
  tracks: TrackItem[];
  done: Record<string, boolean>;
  activeTaskSlug?: string;
  activeUnitSlug?: string;
  onTask: (t: NavTarget) => void;
  onTheory: (t: NavTarget) => void;
  onResetProgress: () => void;
}

/** Содержимое дерева курса. Живёт внутри дока (DockZone задаёт ширину и
 *  resize-ручку); git живёт в своём доке, поэтому здесь его иконки больше нет. */
export function CourseSidebar({ tracks, done, activeTaskSlug, activeUnitSlug, onTask, onTheory, onResetProgress }: CourseSidebarProps) {
  const { togglePanel } = useDock();

  // Extract all unique tags across tasks in this course
  const { allTags, tagCounts } = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const tr of tracks) {
      for (const tp of tr.topics) {
        for (const u of tp.units) {
          for (const t of u.tasks) {
            if (t.tags) {
              for (const tag of t.tags) {
                const trimmed = tag.trim();
                if (trimmed) {
                  counts[trimmed] = (counts[trimmed] || 0) + 1;
                }
              }
            }
          }
        }
      }
    }
    const tags = Object.keys(counts).sort((a, b) =>
      a.localeCompare(b, undefined, { sensitivity: 'base' }),
    );
    return { allTags: tags, tagCounts: counts };
  }, [tracks]);

  const [selectedTags, setSelectedTags] = useState<string[]>([]);

  const isSingleTrackCourse = tracks.length === 1;
  const fullTree = useMemo(() => buildTree(tracks, done), [tracks, done]);

  // Filter tree by selected tags (multiple-filter: matches any of selected tags)
  const tree = useMemo(() => {
    if (selectedTags.length === 0) return fullTree;
    const tagSet = new Set(selectedTags);

    const filterNode = (node: TreeNode): TreeNode | null => {
      if (node.kind === 'task') {
        return node.tags && node.tags.some((t) => tagSet.has(t.trim())) ? node : null;
      }
      if (node.kind === 'theory') {
        return null;
      }
      const filteredChildren: TreeNode[] = [];
      for (const child of node.children) {
        const filteredChild = filterNode(child);
        if (filteredChild) {
          filteredChildren.push(filteredChild);
        }
      }
      if (filteredChildren.length === 0) return null;
      return {
        ...node,
        children: filteredChildren,
        done: filteredChildren.reduce((a, c) => a + c.done, 0),
        total: filteredChildren.reduce((a, c) => a + c.total, 0),
      };
    };

    return fullTree
      .map(filterNode)
      .filter((n): n is TreeNode => n !== null);
  }, [fullTree, selectedTags]);

  const isSingleGroup = isSingleTrackCourse && tree.length === 1 && tree[0].kind === 'group';
  const displayNodes = isSingleGroup ? tree[0].children : tree;
  const startDepth = isSingleGroup ? 1 : 0;

  const totalDone = useMemo(() => tree.reduce((a, n) => a + n.done, 0), [tree]);
  const total = useMemo(() => tree.reduce((a, n) => a + n.total, 0), [tree]);

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

  // When filtering by tags, auto-open all matching branches so user immediately sees results
  useEffect(() => {
    if (selectedTags.length > 0) {
      const nextOpen: Record<string, boolean> = {};
      const openAll = (nodes: TreeNode[]) => {
        for (const n of nodes) {
          if (n.kind === 'group') {
            nextOpen[n.id] = true;
            openAll(n.children);
          }
        }
      };
      openAll(tree);
      setOpen((prev) => ({ ...prev, ...nextOpen }));
    }
  }, [selectedTags, tree]);

  const toggle = (id: string) => setOpen((m) => ({ ...m, [id]: !m[id] }));

  // Expand all / Collapse all helper for top tracks
  const trackIds = useMemo(() => tree.filter((n) => n.kind === 'group').map((n) => n.id), [tree]);
  const allExpanded = trackIds.length > 0 && trackIds.every((id) => open[id]);

  const toggleAll = () => {
    const nextState = !allExpanded;
    const nextOpen: Record<string, boolean> = {};
    const containsActive = (n: TreeNode): boolean => {
      if (n.kind === 'task') return n.nav?.task === activeTaskSlug;
      if (n.kind === 'theory') return n.nav?.unit === activeUnitSlug;
      return n.children.some(containsActive);
    };
    for (const node of tree) {
      if (node.kind !== 'group') continue;
      if (!nextState && containsActive(node)) {
        nextOpen[node.id] = open[node.id] ?? true;
        continue;
      }
      nextOpen[node.id] = nextState;
    }
    setOpen(nextOpen);
  };

  // stable callback for the header close button
  const closeTree = useCallback(() => togglePanel('tree'), [togglePanel]);

  const hasProgress = Object.keys(done).length > 0;

  return (
    <nav className="w-full h-full flex flex-col">
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

        <div className="flex items-center gap-1">
          {!isSingleGroup && trackIds.length > 1 && (
            <button
              type="button"
              onClick={toggleAll}
              title={allExpanded ? 'Свернуть все' : 'Развернуть все'}
              className="w-7 h-7 flex items-center justify-center rounded-md text-tx-3 hover:text-tx-1 hover:bg-bg-3 active:scale-95 transition-all"
            >
              <svg
                width="14"
                height="14"
                viewBox="0 0 16 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="shrink-0"
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
            </button>
          )}
          <button
            type="button"
            onClick={closeTree}
            title="Скрыть содержание"
            className="w-6 h-6 flex items-center justify-center rounded-md text-tx-3 hover:text-tx-1 hover:bg-bg-3 transition-colors cursor-pointer"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      {/* Dynamic Tag Filter (rendered ONLY when course has tagged tasks) */}
      {allTags.length > 0 && (
        <div className="px-3 py-2 border-b border-bdr bg-bg-1/40 select-none space-y-1.5 shrink-0">
          <div className="flex items-center justify-between text-xs">
            <div className="flex items-center gap-1.5 text-tx-3 font-medium">
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
              </svg>
              <span>Фильтр по тегам</span>
              {selectedTags.length > 0 && (
                <span className="text-[10px] font-semibold px-1.5 py-0.2 rounded-full bg-brand/15 text-brand">
                  {selectedTags.length}
                </span>
              )}
            </div>

            {selectedTags.length > 0 && (
              <button
                type="button"
                onClick={() => setSelectedTags([])}
                className="text-[11px] text-tx-3 hover:text-brand transition-colors cursor-pointer"
              >
                Сбросить
              </button>
            )}
          </div>

          <div className="flex flex-wrap gap-1 max-h-24 overflow-y-auto pr-0.5">
            {allTags.map((tag) => {
              const active = selectedTags.includes(tag);
              const count = tagCounts[tag];
              return (
                <button
                  key={tag}
                  type="button"
                  onClick={() => {
                    setSelectedTags((prev) =>
                      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag],
                    );
                  }}
                  className={clsx(
                    'inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-md font-medium border transition-all cursor-pointer',
                    active
                      ? 'bg-brand/15 text-brand border-brand/40 shadow-xs'
                      : 'bg-bg-3 hover:bg-bg-4 text-tx-3 hover:text-tx-2 border-bdr-s',
                  )}
                >
                  <span>{tag}</span>
                  {count !== undefined && (
                    <span className={clsx('text-[10px]', active ? 'text-brand/70' : 'text-tx-3/70')}>
                      {count}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Tree Content */}
      <div className="px-2 py-2.5 space-y-1 flex-1 overflow-y-auto">
        {displayNodes.length === 0 ? (
          <div className="p-4 text-center space-y-2 select-none">
            <p className="text-xs text-tx-3">Нет задач с выбранными тегами</p>
            <button
              type="button"
              onClick={() => setSelectedTags([])}
              className="text-xs text-brand hover:underline font-medium cursor-pointer"
            >
              Сбросить фильтр
            </button>
          </div>
        ) : (
          displayNodes.map((node) => (
            <TreeRow
              key={node.id}
              node={node}
              depth={startDepth}
              open={open}
              toggle={toggle}
              activeTaskSlug={activeTaskSlug}
              activeUnitSlug={activeUnitSlug}
              onTask={onTask}
              onTheory={onTheory}
            />
          ))
        )}
      </div>

      {/* Bottom Progress & Actions Panel */}
      {total > 0 && (
        <div className="p-2.5 border-t border-bdr bg-bg-2 select-none space-y-1.5 shrink-0">
          <div className="flex items-center justify-between text-xs">
            <div className="flex items-center gap-1.5">
              <span className="text-tx-3 text-[11px] font-medium">Прогресс</span>
              <span className={clsx('font-semibold text-[11px]', totalDone === total ? 'text-ok' : 'text-tx-2')}>
                {totalDone === total ? `✓ ${totalDone}/${total}` : `${totalDone}/${total}`} ({Math.round((totalDone / total) * 100)}%)
              </span>
            </div>

            {hasProgress && (
              <button
                type="button"
                onClick={onResetProgress}
                title="Сбросить прогресс"
                className="w-6 h-6 flex items-center justify-center rounded-md text-tx-3 hover:text-err hover:bg-bg-3 active:scale-95 transition-all"
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
                >
                  <path d="M2.5 2v4h4" />
                  <path d="M3.5 10a6 6 0 1 0 1.5-6.5L2.5 6" />
                </svg>
              </button>
            )}
          </div>

          <ProgressBar value={totalDone} max={total} />
        </div>
      )}
    </nav>
  );
}
