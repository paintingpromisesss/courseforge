import { describe, it, expect } from 'vitest';
import { parseHunk, hunkStats } from './diffHunks';

const HUNK = `@@ -1,4 +1,5 @@
 function main() {
-  fmt.Println("hi")
+  fmt.Println("hello")
+  fmt.Println("world")
   return
 }`;

describe('parseHunk', () => {
  it('classifies meta/add/del/ctx rows and strips the diff prefix', () => {
    const rows = parseHunk(HUNK);
    expect(rows).toEqual([
      { kind: 'meta', text: '@@ -1,4 +1,5 @@' },
      { kind: 'ctx', text: 'function main() {' },
      { kind: 'del', text: '  fmt.Println("hi")' },
      { kind: 'add', text: '  fmt.Println("hello")' },
      { kind: 'add', text: '  fmt.Println("world")' },
      { kind: 'ctx', text: '  return' },
      { kind: 'ctx', text: '}' },
    ]);
  });

  it('keeps the no-newline marker as dim meta row', () => {
    const rows = parseHunk('@@ -1 +1 @@\n+a\n\\ No newline at end of file');
    expect(rows[2]).toEqual({ kind: 'meta', text: '\\ No newline at end of file' });
  });
});

describe('hunkStats', () => {
  it('counts added and deleted lines across hunks', () => {
    const { added, deleted } = hunkStats([HUNK, '@@ -9 +9 @@\n-x\n+y\n+z\n unchanged']);
    expect(added).toBe(4);
    expect(deleted).toBe(2);
  });

  it('is zero for an empty hunk list (untracked file)', () => {
    expect(hunkStats([])).toEqual({ added: 0, deleted: 0 });
  });
});
