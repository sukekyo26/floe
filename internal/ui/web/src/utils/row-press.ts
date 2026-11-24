import type { Page } from '@/types/page';
import { plainKey, type KeyPress } from '@/utils/keys';
import { plainPress, type Press } from '@/utils/plain-press';

// What a press on a row's title does: link, the link's own (a new tab); again, the row open beside the list pressed
// again goes whole; one-column, whole by the link, with nothing beside the list; beside, opens it there
export type RowPress = 'link' | 'again' | 'one-column' | 'beside';

export interface RowAt {
  // picked: the row open beside the list, as the press finds it; hasHref: its title links (a link whose file is gone
  // has none); menuOpen: a menu "…" open, which the press closes; selecting: rows selected, whose box it may have missed
  readonly picked: boolean;
  readonly hasHref: boolean;
  readonly menuOpen: boolean;
  readonly selecting: boolean;
  readonly oneColumn: boolean;
}

export function rowPress(press: Press, row: RowAt): RowPress {
  if (!plainPress(press)) return 'link';
  if (row.picked && row.hasHref && !row.menuOpen && !row.selecting) return 'again';
  return row.oneColumn ? 'one-column' : 'beside';
}

// orderOf is what a page open whole goes through by j and k, and what tells a page of the rows, of the entries drawn:
// order leaves out the folders and the links whose file is gone, shown has them all
export const orderOf = (drawn: readonly Page[]): { order: string[]; shown: string[] } => ({
  order: drawn.filter((a) => !a.folder && !a.missing).map((a) => a.slug),
  shown: drawn.map((a) => a.slug),
});

// rowOpens tells a row with a page to open beside the list: with room beside it, of an entry of the page list (a task
// list's row is none), no directory, and of a folder a page of it alone
export const rowOpens = (x: {
  readonly oneColumn: boolean;
  readonly page: Page | undefined;
  readonly dir: boolean;
  readonly file: string;
}): boolean => !x.oneColumn && x.page !== undefined && !x.dir && (!x.page.folder || x.file !== '');

// enterOpens tells Enter on a row's title that opens its page whole: a plain key (Shift too, as the page's keys go),
// not one that ends a composition, on a title that links and opens no tree. It lands on the title, so on no field and
// with no modal open
export const enterOpens = (
  e: KeyPress,
  title: { readonly hasHref: boolean; readonly expandable: boolean; readonly composing: boolean },
): boolean =>
  e.key === 'Enter' &&
  !title.composing &&
  plainKey({ ...e, target: null }, false) &&
  title.hasHref &&
  !title.expandable;

// The row a key moved to (focus-places' moveRow): its entry's slug ('' for a task list's row), a folder's page's file,
// and whether it is a directory of a tree
export interface MovedTo {
  readonly slug: string;
  readonly file: string;
  readonly dir: boolean;
}

// nextRow is the row step from from of count: from none (-1), the first down and the last up; at an end it stays
export function nextRow(count: number, from: number, step: number): number {
  if (from === -1) return step > 0 ? 0 : count - 1;
  return Math.max(0, Math.min(count - 1, from + step));
}
