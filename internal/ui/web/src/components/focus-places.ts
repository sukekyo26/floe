import { flushSync } from 'react-dom';

import { getBoot, screenOf } from '@/config/boot';
import { oneColumn } from '@/lib/platform';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';
import type { ArchiveFocus, TaskOutFocus } from '@/stores/page-actions';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { archiveEscapeTarget, rowAfter, rowNearby } from '@/utils/focus-targets';
import { unseen } from '@/utils/list-view';
import { openKey } from '@/utils/paths';
import { nextRow, rowOpens, type MovedTo } from '@/utils/row-press';
import type { RowSeen } from '@/utils/row-focus';

// Where the focus goes after a page's action, found when called from the key and the screen as it is then: the rows
// of #list, the search modal, the bar. Each finds nothing, and does nothing, where none is there

const rowsNow = (): RowSeen[] =>
  [...document.querySelectorAll('#list > li[data-key]')].map((li) => ({
    key: li.getAttribute('data-key') ?? '',
    titled: li.querySelector('[data-testid=title]') !== null,
  }));

// focusOn gives el the focus, telling whether there was one to take it
export const focusOn = (el: Element | null): boolean => {
  if (!(el instanceof HTMLElement)) return false;
  el.focus();
  return true;
};

// drawThenFocus draws what apply changes at once, then gives the focus to the element of id: what a fold or an open
// button does as it is pressed (ui.html showShelf, showList, showSide). Its callers are event handlers alone, never a
// draw or an effect, where flushSync could not draw
export function drawThenFocus(apply: () => void, id: string): void {
  flushSync(apply);
  focusOn(document.getElementById(id));
}

const focusRow = (key: string | null): boolean =>
  key !== null && focusOn(document.querySelector(`#list > li[data-key="${CSS.escape(key)}"] [data-testid=title]`));

const onTaskPage = () => screenOf(getBoot(), window.location.search) === 'taskPage';
const findOpen = () => document.querySelector('dialog#find[open]') !== null;
const entry = (slug: string) => `entry:${slug}`;

// prepareArchive takes, before the list is read again, the row after the one archived, for Escape on its way back
export function prepareArchive(slug: string): ArchiveFocus {
  const after = rowAfter(rowsNow(), entry(slug));
  return {
    escape: () => {
      focusRow(archiveEscapeTarget(rowsNow(), entry(slug), after, onTaskPage()));
    },
  };
}

// restoreFocus is where the focus goes once a page is restored from its way back, which had it: the search modal's
// field, the page's row, or the frame's archive button (ui.html 3510)
export const restoreFocus = (slug: string) => (): void => {
  if (findOpen()) focusOn(document.getElementById('find-q'));
  else if (!focusRow(entry(slug))) focusOn(document.querySelector('#frame-actions > [data-testid=archive]'));
};

// prepareTaskOut takes, before the page leaves its task, the row next to it; back is the menu it was taken off from.
// Put back, the focus goes to its row, or back; on Escape, back, or the row next to it
// LIMIT: on a task's page ui.html goes on to the category's add button and the task's edit button: P6 adds them here
export function prepareTaskOut(slug: string, back: () => boolean): TaskOutFocus {
  const next = rowNearby(rowsNow(), entry(slug));
  return {
    done: () => {
      if (!focusRow(entry(slug))) back();
    },
    away: () => {
      if (!back()) focusRow(next);
    },
  };
}

// unreadFocus is where the focus goes when the toast of the page shown had it as it was marked unread: the search
// modal's field, or the menu of the bar over the page
export const unreadFocus = (): void => {
  if (findOpen()) focusOn(document.getElementById('find-q'));
  else focusOn(document.querySelector('#frame-actions summary, #read-actions summary'));
};

// menuBack gives the focus back to the menu id's button, telling whether it was there
export const menuBack = (id: string) => (): boolean =>
  focusOn(document.querySelector(`[data-menu-id="${CSS.escape(id)}"] > summary`));

// rowMenuId is the id of a row's menu "…", by its page's slug: a page renamed is found again under its new one
export const rowMenuId = (slug: string): string => `more:${slug}`;

// barMenuId is the id of the menu "…" of the bar of id (over a page, in the frame, the search's foot)
export const barMenuId = (id: string): string => `bar:${id}`;

// moveRow gives the focus to the title of the row step from the one with the focus, or the one open beside the list,
// of the rows drawn with a title, whichever screen (ui.html moveRow): a tree's note has none, and is passed over
export function moveRow(step: number): MovedTo | null {
  const rows = [...document.querySelectorAll('#list > li[data-key]')].filter(
    (li) => li.querySelector('[data-testid=title]') !== null,
  );
  if (rows.length === 0) return null;
  const focused = rows.findIndex((li) => li.contains(document.activeElement));
  const from = focused === -1 ? rows.findIndex((li) => li.getAttribute('aria-current') === 'true') : focused;
  const li = rows[nextRow(rows.length, from, step)];
  const title = li?.querySelector('[data-testid=title]') ?? null;
  if (!li || !focusOn(title)) return null;
  return {
    slug: li.getAttribute('data-slug') ?? '',
    file: li.getAttribute('data-file') ?? '',
    dir: title?.tagName === 'BUTTON',
  };
}

interface BackDeps {
  order?: Order;
  pages?: Pages;
  openPage?: OpenPage;
}

// backToRow gives the focus back to the row a page whole was opened from, come back to (Order's comeBack), and opens
// its version beside the list unless that reads a page left unread. Where the row is no longer seen (the list folded
// as it was, or as it is picked), the focus goes to what opens the list, the one place that can take it then. Called
// once the list is first drawn (useBackToRow) and on the way back from the browser's cache (P8)
// LIMIT: the task list's row is a pin (ui.html 4527): P6 adds it here
export function backToRow({ order = pageOrder, pages = pagePages, openPage = pageOpen }: BackDeps = {}): void {
  const at = order.takeCameFrom();
  if (!at) return;
  const key = at.file ? `page:${openKey(at.slug, at.file)}` : entry(at.slug);
  const title = document.querySelector(`#list > li[data-key="${CSS.escape(key)}"] [data-testid=title]`);
  if (!focusOn(title)) return;
  const page = pages.state.get().entries.find((a) => a.slug === at.slug);
  if (
    page &&
    !unseen(page) &&
    rowOpens({ oneColumn: oneColumn(), page, dir: false, file: at.file }) &&
    !openPage.isPicked(at)
  )
    openPage.picked.set(at);
  if (document.activeElement !== title) focusOn(document.getElementById('list-show'));
}
