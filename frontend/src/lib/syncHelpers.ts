// Pure helpers for the cloud-sync settings UI (unit-tested, no React).

export interface CourseSyncRow {
  dir: string;
  title: string;
  isImport: boolean;
  excluded: boolean;
}

// buildSyncRows merges the course list, sources and the exclude list into
// display rows. Git-imported courses are marked (they sync from their own
// remote) and always excluded from the vault snapshot.
export function buildSyncRows(
  courses: { dir: string; title: string }[],
  sources: Record<string, unknown>,
  exclude: string[],
): CourseSyncRow[] {
  const excluded = new Set(exclude);
  return courses.map((c) => {
    const isImport = Object.prototype.hasOwnProperty.call(sources, c.dir);
    return {
      dir: c.dir,
      title: c.title,
      isImport,
      excluded: isImport || excluded.has(c.dir),
    };
  });
}

// toggleExclude computes the next exclude list after a checkbox toggle.
// Imported courses can never be toggled into the vault.
export function toggleExclude(rows: CourseSyncRow[], dir: string, next: boolean): string[] {
  const row = rows.find((r) => r.dir === dir);
  if (!row || row.isImport) return rows.filter((r) => r.excluded && !r.isImport).map((r) => r.dir);
  const current = new Set(rows.filter((r) => r.excluded && !r.isImport).map((r) => r.dir));
  if (next) {
    current.delete(dir);
  } else {
    current.add(dir);
  }
  return [...current];
}

export function formatSyncTime(iso: string | undefined): string {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString('ru-RU');
  } catch {
    return iso;
  }
}

// The confirm question shown before a rollback.
export function rollbackQuestion(commit: string): string {
  return `Откатить состояние к коммиту ${commit.slice(0, 7)}? Текущие изменения будут закоммичены как отдельный откат.`;
}
