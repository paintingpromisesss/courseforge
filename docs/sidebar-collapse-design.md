# Sidebar Collapse — Design

## Problem

The course tree has a fixed 5-level hierarchy: Course → Track → Topic → Unit → Task.
The sidebar (`frontend/src/pages/CoursePage.tsx`) renders all intermediate levels
unconditionally. When an intermediate level has a single child, the nesting carries
no information and produces redundant rows (e.g. Track "Defer" → Topic "Defer" → one
task). Meaningful splits (multiple topics or units) must still be shown.

## Understanding Summary

- **What:** Flatten redundant nesting in the sidebar tree. Presentation only.
- **Why:** Rigid hierarchy forces every course through Track→Topic→Unit even when a
  level has one child, creating meaningless double-rows.
- **Who:** Learners navigating courses. Authors keep authoring exactly as today.
- **Constraints:** No backend, schema, migration, or route changes.
- **Non-goals:** Recursive/flexible course format; optional manifest levels.

## Collapse Rule

Any node that is the **only child** of its parent is hidden. A chain of only-children
collapses into a single row carrying the **topmost** node's title. A node with siblings
always renders.

## Assumptions (confirmed)

1. Rule is **only-child** based, not duplicate-title based — collapses even when titles
   differ (Track "Defer" → Topic "Введение" → row titled "Defer").
2. Applies uniformly at Track, Topic, and Unit levels.
3. Bottom of a collapsed chain keeps its nature: unit-with-tasks → expandable row with
   tasks; theory-only unit → leaf navigating to theory.
4. Progress counts (`done/total`) attach to the surviving row, summing everything below.
5. Routing untouched — clicks build the full
   `/tracks/x/topics/y/units/z/tasks/w` path; all slugs still exist in data.
6. A single track is **not** dropped: it collapses with its single-child chain into
   one surviving row carrying the track's title (uniform topmost-wins). It is not
   removed with its topics promoted to the top — that would discard the track title,
   conflicting with the topmost-title rule.

## Design

### 1. Data model — `buildTree(tracks)`

A pure function maps the API response (`TrackItem[]`) into a normalized view tree with
redundant levels already removed. Rendering becomes dumb.

```ts
interface TreeNode {
  kind: 'group' | 'tasks' | 'theory' | 'task';
  title: string;          // topmost title of the collapsed chain
  children: TreeNode[];
  done: number;
  total: number;
  nav?: { track: string; topic: string; unit: string; task?: string }; // full slugs
  doneFlag?: boolean;     // leaves: completed?
}
```

Build by walking Track → Topic → Unit → tasks. At each container level:
- build children recursively;
- if the node has exactly one group child **and** carries no own content (not a
  theory-bearing unit) → return the child, but keep the current (upper) node's title;
- otherwise return the node with its children.

`nav` is accumulated top-down so the full slug path survives even through hidden nodes.
`done/total` are accumulated bottom-up during the walk — this replaces the current
`countTasks` / `doneInTrack` / `countTopicTasks` / `doneInTopic` helpers.

### 2. Render + state

One recursive `<TreeRow node depth>` replaces the four hardcoded nested `.map` blocks.
Indent is driven by `depth`, not by node type.

- `group` / `tasks` → row with `▶` chevron, recurse on `children` when open.
- `theory` → leaf, `✓/·` icon from `done[unitSlug]`, click → `onTheory(nav)`.
- `task` → leaf, `✓/·` icon from `done[taskSlug]`, click → `onTask(nav)`.

Expansion state becomes a single `Record<string, boolean>` keyed by a stable node id =
the node's slug path (`track/topic/unit`), not a bare slug — after collapse the upper
title may belong to a track while expanding a unit's contents, so the path key avoids
collisions. Replaces the three per-level dictionaries.

`findActivePath` is rewritten to return the set of node ids from root to the active
leaf, used to initialize the open set. Top-level groups default open, as today.

Progress bar stays on top-level rows with `total > 0`, fed by `node.done/node.total`.

### 3. Edge cases

1. Unit with **both** theory and tasks → not collapsed; renders a `group` with children
   `[theory, ...tasks]`. Real content, not an empty container.
2. Topic with one unit, unit with one task → whole chain collapses to one task leaf
   under the topmost title.
3. Theory-only unit as the sole child of a topic → leaf takes the **topmost** title;
   `nav` points to the unit's theory.
4. Empty node (0 tasks, 0 theory) → filtered out (defensive; format forbids it today).
5. Course with one track → track collapses with its chain into one row keeping the
   track's title (not removed; topmost-wins applies uniformly).

### Testing

Vitest unit tests on `buildTree` (pure function):
- single-child chain → one row, topmost title;
- siblings at a level → no collapse;
- differing titles in a chain → topmost survives;
- `done/total` summed correctly;
- `nav` retains full slugs after collapse;
- unit with theory+tasks → not collapsed;
- single track → collapses to one row keeping the track title;
- empty groups dropped.

Implemented in `frontend/src/lib/buildTree.test.ts` (Vitest, `npm test`).

No render snapshot needed — logic lives entirely in `buildTree`.

## Decision Log

| Decision | Alternatives | Why |
|---|---|---|
| UI-only collapse | Recursive flexible format; hybrid optional levels | Lowest risk; existing courses, backend, API, routes untouched. |
| Collapse rule = only-child | Collapse only on duplicate titles | Matches the actual intent ("no point nesting when nothing else is there"); duplicate-title is a narrower coincidence. |
| Keep topmost title | Deepest title; join with separator; smart dedupe | Predictable; topmost is usually the meaningful label; separator is noisy in a w-64 sidebar. |
| Single `buildTree` + recursive row | Patch the existing 4-level render in place | Isolates all collapse logic in one testable pure function; render stays dumb. |
| Node id = slug path | Bare per-level slug dictionaries | Stable, collision-free after levels are hidden/merged. |
| Routing unchanged | Shorten URLs to match collapsed view | Slugs still exist in data; avoids backend/route churn (non-goal). |
| Single track collapses (keeps title), not removed | Drop the track row, promote topics to top | Removing it would discard the track title, conflicting with topmost-wins; uniform rule is simpler. |
