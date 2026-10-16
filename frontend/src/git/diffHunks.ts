// Parsing of the unified-diff hunks the backend serves (one string per hunk,
// starting with @@ — see backend GitDiffResp). Pure and testable.

export type DiffLineKind = 'meta' | 'ctx' | 'add' | 'del';

export interface DiffLine {
  kind: DiffLineKind;
  text: string;
}

/** Split one hunk's text into colored rows. */
export function parseHunk(hunk: string): DiffLine[] {
  const out: DiffLine[] = [];
  for (const line of hunk.split('\n')) {
    if (line.startsWith('@@')) {
      out.push({ kind: 'meta', text: line });
    } else if (line.startsWith('+')) {
      out.push({ kind: 'add', text: line.slice(1) });
    } else if (line.startsWith('-')) {
      out.push({ kind: 'del', text: line.slice(1) });
    } else if (line.startsWith('\\')) {
      // "\ No newline at end of file" — noise, keep it dim but visible
      out.push({ kind: 'meta', text: line });
    } else if (line.startsWith(' ')) {
      out.push({ kind: 'ctx', text: line.slice(1) });
    } else if (line !== '' || out.length === 0) {
      // tolerate slightly malformed hunks: pass through as context
      out.push({ kind: 'ctx', text: line });
    }
  }
  return out;
}

/** Line churn shown on the file row: +N −M across all hunks. */
export function hunkStats(hunks: string[]): { added: number; deleted: number } {
  let added = 0;
  let deleted = 0;
  for (const hunk of hunks) {
    for (const line of hunk.split('\n')) {
      if (line.startsWith('+')) added++;
      else if (line.startsWith('-')) deleted++;
    }
  }
  return { added, deleted };
}
