import { describe, it, expect } from 'vitest';
import { computeSplitDiff } from './lineDiff';

describe('computeSplitDiff', () => {
  it('marks identical files as all-equal rows', () => {
    const rows = computeSplitDiff('a\nb\nc', 'a\nb\nc');
    expect(rows).toEqual([
      { kind: 'eq', n: 1, text: 'a' },
      { kind: 'eq', n: 2, text: 'b' },
      { kind: 'eq', n: 3, text: 'c' },
    ]);
  });

  it('pairs a modified block so «было» и «стало» sit side by side', () => {
    const rows = computeSplitDiff('one\nold\ntwo\nthree', 'one\nnew\ntwo\nthree');
    // eq(one) → del(old), add(new) → eq(two), eq(three)
    expect(rows).toEqual([
      { kind: 'eq', n: 1, text: 'one' },
      { kind: 'del', n: 2, text: 'old' },
      { kind: 'add', n: 2, text: 'new' },
      { kind: 'eq', n: 3, text: 'two' },
      { kind: 'eq', n: 4, text: 'three' },
    ]);
  });

  it('handles pure additions', () => {
    const rows = computeSplitDiff('a\nb', 'a\nb\nc\nd');
    const adds = rows.filter((r) => r.kind === 'add');
    expect(adds.map((r) => r.text)).toEqual(['c', 'd']);
    expect(rows.filter((r) => r.kind === 'del')).toHaveLength(0);
  });

  it('handles pure deletions', () => {
    const rows = computeSplitDiff('a\nb\nc\nd', 'a\nc');
    const dels = rows.filter((r) => r.kind === 'del');
    expect(dels.map((r) => r.text)).toEqual(['b', 'd']);
  });

  it('treats empty old version as all additions', () => {
    const rows = computeSplitDiff('', 'hello');
    expect(rows).toEqual([{ kind: 'add', n: 1, text: 'hello' }]);
  });

  it('falls back to a whole-file replace block for huge inputs', () => {
    const bigLine = 'x';
    const oldHuge = Array(3000).fill(bigLine).join('\n');
    const newHuge = Array(3000).fill(bigLine + 'y').join('\n');
    const rows = computeSplitDiff(oldHuge, newHuge);
    expect(rows.filter((r) => r.kind === 'del')).toHaveLength(3000);
    expect(rows.filter((r) => r.kind === 'add')).toHaveLength(3000);
    expect(rows.filter((r) => r.kind === 'eq')).toHaveLength(0);
  });
});
