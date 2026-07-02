import { describe, it, expect } from 'vitest';
import { buildTree, initialOpen } from './buildTree';
import type { TrackItem, TaskItem, UnitItem, TopicItem } from '../api/types';

const task = (slug: string, title = slug): TaskItem => ({ slug, title, languages: ['go'] });
const unit = (slug: string, tasks: TaskItem[], has_theory = false, title = slug): UnitItem => ({
  slug, title, has_theory, tasks,
});
const topic = (slug: string, units: UnitItem[], title = slug): TopicItem => ({
  slug, title, description: '', units,
});
const track = (slug: string, topics: TopicItem[], title = slug): TrackItem => ({
  slug, title, description: '', topics,
});

describe('buildTree', () => {
  it('collapses a single-child chain to one row with the topmost title', () => {
    const tracks = [track('defer', [topic('defer-t', [unit('u', [task('t1')])])], 'Defer')];
    const tree = buildTree(tracks, {});
    expect(tree).toHaveLength(1);
    // track -> topic -> unit -> single task all collapse to one task leaf, topmost title wins
    expect(tree[0].kind).toBe('task');
    expect(tree[0].title).toBe('Defer');
    expect(tree[0].nav).toEqual({ track: 'defer', topic: 'defer-t', unit: 'u', task: 't1' });
  });

  it('keeps nesting when a level has siblings', () => {
    const tracks = [
      track('w1', [
        topic('strings', [unit('su', [task('a'), task('b')], false, 'Строки')], 'Работа со строками'),
        topic('pointers', [unit('pu', [task('c')])], 'Указатели'),
      ], 'Неделя 1'),
    ];
    const tree = buildTree(tracks, {});
    expect(tree[0].title).toBe('Неделя 1');
    expect(tree[0].kind).toBe('group');
    expect(tree[0].children).toHaveLength(2); // two topics survive
    expect(tree[0].children.map((c) => c.title)).toEqual(['Работа со строками', 'Указатели']);
    // pointers topic -> single unit -> single task collapses to one task leaf
    expect(tree[0].children[1].kind).toBe('task');
  });

  it('topmost title survives even when chain titles differ', () => {
    const tracks = [track('defer', [topic('intro', [unit('u', [task('t1'), task('t2')])])], 'Defer')];
    const tree = buildTree(tracks, {});
    // track -> topic both single children, unit has 2 tasks -> group of 2, titled topmost "Defer"
    expect(tree[0].kind).toBe('group');
    expect(tree[0].title).toBe('Defer');
    expect(tree[0].children).toHaveLength(2);
  });

  it('sums done/total onto the surviving row', () => {
    const tracks = [
      track('w1', [
        topic('a', [unit('ua', [task('x'), task('y')])], 'A'),
        topic('b', [unit('ub', [task('z')])], 'B'),
      ], 'Week'),
    ];
    const tree = buildTree(tracks, { x: true, z: true });
    expect(tree[0].total).toBe(3);
    expect(tree[0].done).toBe(2);
  });

  it('retains full slugs in nav after collapse', () => {
    const tracks = [track('tr', [topic('tp', [unit('un', [task('tk')])])])];
    const leaf = buildTree(tracks, {})[0];
    expect(leaf.nav).toEqual({ track: 'tr', topic: 'tp', unit: 'un', task: 'tk' });
  });

  it('omits the theory row when a unit also has tasks (theory lives in the task tab)', () => {
    const tracks = [track('tr', [topic('tp', [unit('un', [task('a'), task('b')], true, 'Unit')])])];
    const tree = buildTree(tracks, {});
    // unit group survives (2 tasks), but theory is not given its own row
    expect(tree[0].kind).toBe('group');
    expect(tree[0].title).toBe('tr'); // topmost track title
    expect(tree[0].children.map((c) => c.kind)).toEqual(['task', 'task']);
  });

  it('theory-only unit collapses to a theory leaf with topmost title', () => {
    const tracks = [track('tr', [topic('tp', [unit('un', [], true, 'Intro')])], 'Track')];
    const tree = buildTree(tracks, { un: true });
    expect(tree[0].kind).toBe('theory');
    expect(tree[0].title).toBe('Track');
    expect(tree[0].doneFlag).toBe(true);
    expect(tree[0].nav).toEqual({ track: 'tr', topic: 'tp', unit: 'un' });
  });

  it('collapses a single track (no sibling tracks) onto its surviving row', () => {
    const tracks = [
      track('only', [
        topic('a', [unit('ua', [task('x')])], 'A'),
        topic('b', [unit('ub', [task('y')])], 'B'),
      ], 'Sole'),
    ];
    const tree = buildTree(tracks, {});
    // single track with 2 topics stays as one group titled "Sole"
    expect(tree).toHaveLength(1);
    expect(tree[0].title).toBe('Sole');
    expect(tree[0].children).toHaveLength(2);
  });

  it('drops empty groups', () => {
    const tracks = [track('empty', [])];
    expect(buildTree(tracks, {})).toHaveLength(0);
  });
});

describe('initialOpen', () => {
  it('opens top-level groups and ancestors of the active task', () => {
    const tracks = [
      track('w1', [
        topic('a', [unit('ua', [task('x'), task('y')])], 'A'),
        topic('b', [unit('ub', [task('z'), task('w')])], 'B'),
      ], 'Week'),
    ];
    const tree = buildTree(tracks, {});
    const open = initialOpen(tree, 'z');
    expect(open[tree[0].id]).toBe(true); // top-level group open
    const topicB = tree[0].children[1];
    expect(open[topicB.id]).toBe(true); // ancestor of active task z open
  });
});
