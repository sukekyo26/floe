import type { Page } from '@/types/page';

// What a part of the shelf shows: its rows, each with its count where the rest is how it is now
export interface Tally {
  // rows are the values of the entries the view has, each with its number there to order them by
  rows: ReadonlyMap<string, number>;
  // n is how many of left have value: what pressing it would leave
  n: (value: string) => number;
}

const count = (entries: readonly Page[], values: (a: Page) => readonly string[]): Map<string, number> => {
  const counts = new Map<string, number>();
  for (const a of entries) for (const value of values(a)) counts.set(value, (counts.get(value) ?? 0) + 1);
  return counts;
};

// tally is a part of the shelf: its rows are those of list (the view's), and what is picked of it stays at 0, so that
// no row jumps away as the rest narrows; its counts are of left, the entries the rest of the shelf, the view and the
// search leave (the page list), or list again (the task list, of its project alone)
export function tally(
  list: readonly Page[],
  values: (a: Page) => readonly string[],
  picked: string,
  left: readonly Page[],
): Tally {
  const rows = count(list, values);
  if (picked && !rows.has(picked)) rows.set(picked, 0);
  const counts = count(left, values);
  return { rows, n: (value) => counts.get(value) ?? 0 };
}
