import { describe, it, expect } from 'vitest';
import { buildSyncRows, toggleExclude, formatSyncTime, rollbackQuestion } from './syncHelpers';

describe('buildSyncRows', () => {
  it('marks imported courses as excluded and untoggleable', () => {
    const rows = buildSyncRows(
      [{ dir: 'plain', title: 'Plain' }, { dir: 'imp', title: 'Imported' }],
      { imp: { repo: 'https://github.com/x/y' } },
      ['plain'],
    );
    expect(rows).toEqual([
      { dir: 'plain', title: 'Plain', isImport: false, excluded: true },
      { dir: 'imp', title: 'Imported', isImport: true, excluded: true },
    ]);
  });

  it('empty sources and exclude → all synced', () => {
    const rows = buildSyncRows([{ dir: 'a', title: 'A' }], {}, []);
    expect(rows[0].excluded).toBe(false);
  });
});

describe('toggleExclude', () => {
  const rows = buildSyncRows(
    [{ dir: 'a', title: 'A' }, { dir: 'b', title: 'B' }, { dir: 'imp', title: 'I' }],
    { imp: {} },
    ['b'],
  );

  it('unchecking a synced course adds it to exclude', () => {
    expect(toggleExclude(rows, 'a', false)).toContain('a');
  });

  it('checking an excluded course removes it from exclude', () => {
    expect(toggleExclude(rows, 'b', true)).not.toContain('b');
  });

  it('imported courses stay out of the exclude list', () => {
    expect(toggleExclude(rows, 'imp', true)).not.toContain('imp');
  });
});

describe('formatSyncTime', () => {
  it('em-dash for empty', () => {
    expect(formatSyncTime(undefined)).toBe('—');
  });
  it('keeps unparseable strings as-is', () => {
    expect(formatSyncTime('not a date')).toBe('not a date');
  });
});

describe('rollbackQuestion', () => {
  it('mentions the short commit hash', () => {
    const q = rollbackQuestion('abcdef1234567890');
    expect(q).toContain('abcdef1');
  });
});
