import type { Screen } from '@/types/screen';

export interface ColumnWidths {
  shelf: number;
  list: number;
  shelfMin: number;
  shelfMax: number;
  listMin: number;
  listMax: number;
}

const within = (value: number, min: number, max: number): number => Math.round(Math.min(Math.max(value, min), max));

// columnWidths is the two columns' widths in px within the bounds of a window innerWidth wide, and those bounds.
// Defaults and bounds go in rem, so that a larger text in the browser does not narrow them; the list is held off the
// page beside it by the room the page keeps at its narrowest where the filters are a row (56rem less the list's 22rem,
// less the room round it). The shelf takes nothing where it is not shown
export function columnWidths(
  kept: { readonly shelf: number | null; readonly list: number | null },
  at: { readonly shelfShown: boolean; readonly innerWidth: number; readonly rem: number },
): ColumnWidths {
  const { rem } = at;
  const shelfMin = Math.round(10 * rem);
  const shelfMax = Math.round(20 * rem);
  const listMin = Math.round(20 * rem);
  const shelf = at.shelfShown ? within(kept.shelf ?? 12.5 * rem, shelfMin, shelfMax) : 0;
  const listMax = Math.max(listMin, Math.round(at.innerWidth - shelf - 32.5 * rem));
  return { shelf, list: within(kept.list ?? 25.5 * rem, listMin, listMax), shelfMin, shelfMax, listMin, listMax };
}

// listShown tells whether the list shows beside the page: the task list has nothing but the list to fold it into, and a
// task's page keeps its rows until a page is there to take their place
export const listShown = (screen: Screen, listOpen: boolean, picked: boolean): boolean =>
  screen === 'taskList' || listOpen || (screen === 'taskPage' && !picked);
