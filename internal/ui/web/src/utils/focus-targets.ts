import type { RowSeen } from '@/utils/row-focus';

// rowAfter is the key of the first row with a title after key, as the rows are drawn now, or null
export function rowAfter(rows: readonly RowSeen[], key: string): string | null {
  const at = rows.findIndex((r) => r.key === key);
  if (at === -1) return null;
  return rows.slice(at + 1).find((r) => r.titled)?.key ?? null;
}

// rowNearby is the key of the row with a title after key, or, with none after it, before it: where the focus goes on as
// a page leaves its task
export function rowNearby(rows: readonly RowSeen[], key: string): string | null {
  const titled = rows.filter((r) => r.titled || r.key === key);
  const at = titled.findIndex((r) => r.key === key);
  if (at === -1) return null;
  return (titled[at + 1] ?? titled[at - 1])?.key ?? null;
}

// archiveEscapeTarget is where Escape on an archive's way back leaves the focus: on a task's page its own row, which
// stays dimmed; else the row that was after it before the list was read again; else the last row with a title
export function archiveEscapeTarget(
  rows: readonly RowSeen[],
  own: string,
  after: string | null,
  taskPage: boolean,
): string | null {
  const titled = rows.filter((r) => r.titled);
  if (taskPage && titled.some((r) => r.key === own)) return own;
  if (after !== null && titled.some((r) => r.key === after)) return after;
  return titled.at(-1)?.key ?? null;
}
