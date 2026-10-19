// Split («было — стало») pairing of two file versions.
// Classic LCS over lines; large inputs fall back to a whole-file replace
// block so the diff view never freezes the UI.

export type SplitRow =
  | { kind: 'eq'; n: number; text: string }   // строка есть в обеих версиях
  | { kind: 'del'; n: number; text: string }  // осталась только в «было»
  | { kind: 'add'; n: number; text: string }; // появилась только в «стало»

const MAX_LCS_LINES = 2500;

export function computeSplitDiff(oldText: string, newText: string): SplitRow[] {
  const a = oldText.length === 0 ? [] : oldText.split('\n');
  const b = newText.length === 0 ? [] : newText.split('\n');

  // Cheap fast path: identical content.
  if (a.length === b.length && a.every((v, i) => v === b[i])) {
    return a.map((text, i) => ({ kind: 'eq' as const, n: i + 1, text }));
  }

  // Fallback for very large files: treat as one big replace block.
  if (a.length > MAX_LCS_LINES || b.length > MAX_LCS_LINES) {
    return [
      ...a.map((text, i) => ({ kind: 'del' as const, n: i + 1, text })),
      ...b.map((text, i) => ({ kind: 'add' as const, n: i + 1, text })),
    ];
  }

  // LCS table: lcs[i][j] = length of the longest common subsequence of a[i:], b[j:].
  const m = a.length;
  const n = b.length;
  const lcs: Uint32Array[] = Array.from({ length: m + 1 }, () => new Uint32Array(n + 1));
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }

  const rows: SplitRow[] = [];
  const pendingDel: SplitRow[] = [];
  const pendingAdd: SplitRow[] = [];
  const flushPair = () => {
    // Interleave delete/add blocks so paired rows sit side by side.
    const len = Math.max(pendingDel.length, pendingAdd.length);
    for (let k = 0; k < len; k++) {
      if (pendingDel[k]) rows.push(pendingDel[k]);
      if (pendingAdd[k]) rows.push(pendingAdd[k]);
    }
    pendingDel.length = 0;
    pendingAdd.length = 0;
  };

  let i = 0;
  let j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) {
      flushPair();
      rows.push({ kind: 'eq', n: i + 1, text: a[i] });
      i++;
      j++;
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      pendingDel.push({ kind: 'del', n: i + 1, text: a[i] });
      i++;
    } else {
      pendingAdd.push({ kind: 'add', n: j + 1, text: b[j] });
      j++;
    }
  }
  while (i < m) {
    pendingDel.push({ kind: 'del', n: i + 1, text: a[i] });
    i++;
  }
  while (j < n) {
    pendingAdd.push({ kind: 'add', n: j + 1, text: b[j] });
    j++;
  }
  flushPair();
  return rows;
}
