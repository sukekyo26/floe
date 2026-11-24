// What in a row may hold the focus as the rows are drawn anew
export type FocusOn = 'title' | 'summary' | 'star' | 'archive';

export interface Focused {
  key: string;
  on: FocusOn;
}

// A row as drawn: its key, and whether it has a title to take the focus
export interface RowSeen {
  key: string;
  titled: boolean;
}

const STAYS = new Set<FocusOn>(['title', 'summary']);

// focusTarget is where the focus goes once the rows are drawn anew, of what had it before: the same in its row where
// the row stayed, and a button only if the row kept its place among the rows there before and after (on a task's
// page archiving moves a row on, and a key held down or a press more on the same button would go on to the next
// page). Else the title of the row after it as they were, or of the last row; null where no row has a title
export function focusTarget(was: readonly string[], now: readonly RowSeen[], focused: Focused): Focused | null {
  const nowKeys = new Set(now.map((r) => r.key));
  const wasKeys = new Set(was);
  const place = (keys: readonly string[], kept: ReadonlySet<string>) =>
    keys.filter((k) => kept.has(k)).indexOf(focused.key);
  if (
    nowKeys.has(focused.key) &&
    (STAYS.has(focused.on) ||
      place(was, nowKeys) ===
        place(
          now.map((r) => r.key),
          wasKeys,
        ))
  )
    return focused;
  const titled = new Set(now.filter((r) => r.titled).map((r) => r.key));
  const next = was.slice(was.indexOf(focused.key) + 1).find((k) => titled.has(k));
  const to = next ?? now.findLast((r) => r.titled)?.key;
  return to === undefined ? null : { key: to, on: 'title' };
}
