import { useState, useRef, useEffect, useMemo, useCallback, memo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useParams, useNavigate, useLocation, useOutlet } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { ProgressBar } from '../components/ui/ProgressBar';
import { DifficultyBadge } from '../components/ui/DifficultyBadge';
import type { TrackItem } from '../api/types';
import { buildTree, type TreeNode, type NavTarget } from '../lib/buildTree';

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

function computeAutoFitWidth(
  tree: TreeNode[],
  open: Record<string, boolean>,
  isSingleGroup: boolean = false,
): number {
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
      const diffW = node.difficulty ? 48 : 0;
      rowWidth = 16 + indent + 22 + titleW + diffW + 36;
    }

    if (rowWidth > maxW) maxW = rowWidth;

    if (node.kind === 'group' && (depth === 0 ? open[node.id] : true)) {
      for (const child of node.children) {
        traverse(child, depth + 1);
      }
    }
  };

  const roots = isSingleGroup && tree.length === 1 && tree[0].kind === 'group' ? tree[0].children : tree;
  const startDepth = isSingleGroup && tree.length === 1 && tree[0].kind === 'group' ? 1 : 0;

  for (const root of roots) {
    traverse(root, startDepth);
  }

  return Math.min(Math.max(Math.ceil(maxW), 260), 650);
}

interface SidebarProps {
  title: string;
  tracks: TrackItem[];
  done: Record<string, boolean>;
  activeTaskSlug?: string;
  activeUnitSlug?: string;
  onTask: (t: NavTarget) => void;
  onTheory: (t: NavTarget) => void;
  onResetProgress: () => void;
}

function Sidebar({ tracks, done, activeTaskSlug, activeUnitSlug, onTask, onTheory, onResetProgress }: SidebarProps) {
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
        return node.tags && node.tags.some((t) => tagSet.has(t)) ? node : null;
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
    return computeAutoFitWidth(tree, initOpen, isSingleTrackCourse);
  });

  const [isDragging, setIsDragging] = useState(false);

  const measureRealDom = useCallback((): number => {
    if (!treeRef.current) return computeAutoFitWidth(tree, open, isSingleGroup);
    const container = treeRef.current;
    const containerRect = container.getBoundingClientRect();
    let maxNeeded = 270;

    const rows = container.querySelectorAll<HTMLElement>('button, .select-none');
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    if (!ctx) return computeAutoFitWidth(tree, open, isSingleGroup);

    rows.forEach((row) => {
      const textSpan = row.querySelector<HTMLElement>('.truncate');
      if (!textSpan || !textSpan.textContent) return;
      const rowRect = row.getBoundingClientRect();
      const leftOffset = rowRect.left - containerRect.left;

      const computed = window.getComputedStyle(textSpan);
      ctx.font = `${computed.fontWeight} ${computed.fontSize} ${computed.fontFamily}`;
      const naturalTextWidth = ctx.measureText(textSpan.textContent).width;

      let badgeW = 0;
      // Progress badge in groups (has .rounded-full, exclude tiny topic indicator .w-1)
      const progressBadge = row.querySelector<HTMLElement>('.rounded-full:not(.w-1)');
      if (progressBadge && progressBadge.offsetWidth > 10) {
        badgeW += progressBadge.offsetWidth + 8;
      }

      // Difficulty badge in task rows
      const diffBadge = row.querySelector<HTMLElement>('.difficulty-badge, [title^="Сложность"]');
      if (diffBadge) {
        badgeW += diffBadge.offsetWidth + 8;
      }

      const iconW = 24;

      const totalNeeded = leftOffset + iconW + naturalTextWidth + badgeW + 36;
      if (totalNeeded > maxNeeded) {
        maxNeeded = totalNeeded;
      }
    });

    return Math.min(Math.max(Math.ceil(maxNeeded), 260), 650);
  }, [tree, open, isSingleGroup]);

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

  const hasProgress = Object.keys(done).length > 0;

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
        <div ref={treeRef} className="px-2 py-2.5 space-y-1 flex-1 overflow-y-auto">
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

  const outlet = useOutlet({ mainRef } satisfies CoursePageContext);

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
        transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
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
      <main
        ref={mainRef}
        className="flex-1 h-full overflow-hidden relative"
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
    </div>
  );
}
