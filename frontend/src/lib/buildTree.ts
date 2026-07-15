import type { TrackItem, TopicItem, UnitItem, TaskItem } from '../api/types';

export type NodeKind = 'group' | 'task' | 'theory';

export interface NavTarget {
  track: string;
  topic: string;
  unit: string;
  task?: string;
}

export interface TreeNode {
  id: string;
  kind: NodeKind;
  title: string;
  children: TreeNode[];
  done: number;
  total: number;
  nav?: NavTarget;
  doneFlag?: boolean;
}

type Done = Record<string, boolean>;

function group(id: string, title: string, children: TreeNode[]): TreeNode {
  return {
    id,
    kind: 'group',
    title,
    children,
    done: children.reduce((a, c) => a + c.done, 0),
    total: children.reduce((a, c) => a + c.total, 0),
  };
}

function taskLeaf(base: NavTarget, t: TaskItem, done: Done): TreeNode {
  const flag = !!done[t.slug];
  return {
    id: `${base.track}/${base.topic}/${base.unit}/${t.slug}`,
    kind: 'task',
    title: t.title,
    children: [],
    done: flag ? 1 : 0,
    total: 1,
    nav: { ...base, task: t.slug },
    doneFlag: flag,
  };
}

function theoryLeaf(base: NavTarget, title: string, done: Done): TreeNode {
  // A theory-only unit counts toward group N/M totals like a task does; its
  // completion is stored under the unit slug (see TheoryPage.markTheoryDone).
  const flag = !!done[base.unit];
  return {
    id: `${base.track}/${base.topic}/${base.unit}/theory`,
    kind: 'theory',
    title,
    children: [],
    done: flag ? 1 : 0,
    total: 1,
    nav: base,
    doneFlag: flag,
  };
}

function buildUnit(tr: TrackItem, tp: TopicItem, u: UnitItem, done: Done): TreeNode {
  const base: NavTarget = { track: tr.slug, topic: tp.slug, unit: u.slug };
  const id = `${tr.slug}/${tp.slug}/${u.slug}`;
  const tasks = u.tasks.map((t) => taskLeaf(base, t, done));

  // A unit's theory shares its tasks' route; the TaskPage shows it as a "Теория"
  // tab, so it gets no separate sidebar row when tasks exist. Only a theory-only
  // unit needs its own leaf.
  if (tasks.length > 0) return group(id, u.title, tasks);
  if (u.has_theory) return theoryLeaf(base, u.title, done);
  return group(id, u.title, tasks); // empty unit -> filtered out
}

function buildTrack(tr: TrackItem, done: Done): TreeNode {
  const topics = tr.topics.map((tp) =>
    group(
      `${tr.slug}/${tp.slug}`,
      tp.title,
      tp.units.map((u) => buildUnit(tr, tp, u, done)),
    ),
  );
  return group(tr.slug, tr.title, topics);
}

// collapse hides any group that is the only child of its parent; the surviving
// row keeps the topmost (outermost) node's title. Outer overwrites inner, so the
// topmost wins up the chain. Empty groups are dropped.
function collapse(node: TreeNode): TreeNode {
  if (node.kind !== 'group') return node;
  const children = node.children.map(collapse);
  if (children.length === 1) {
    return { ...children[0], title: node.title };
  }
  return { ...node, children };
}

/** Build the flattened sidebar tree from a course's tracks. */
export function buildTree(tracks: TrackItem[], done: Done): TreeNode[] {
  return tracks
    .map((tr) => collapse(buildTrack(tr, done)))
    .filter((n) => n.kind !== 'group' || n.children.length > 0);
}

/** Ids of groups to open initially: every top-level group plus all ancestors of the active leaf. */
export function initialOpen(
  nodes: TreeNode[],
  activeTaskSlug?: string,
  activeUnitSlug?: string,
): Record<string, boolean> {
  const open: Record<string, boolean> = {};
  for (const n of nodes) if (n.kind === 'group') open[n.id] = true;

  const mark = (node: TreeNode): boolean => {
    if (node.kind === 'task') return !!activeTaskSlug && node.nav?.task === activeTaskSlug;
    if (node.kind === 'theory') return !!activeUnitSlug && node.nav?.unit === activeUnitSlug;
    let hit = false;
    for (const c of node.children) if (mark(c)) hit = true;
    if (hit) open[node.id] = true;
    return hit;
  };
  nodes.forEach(mark);
  return open;
}
