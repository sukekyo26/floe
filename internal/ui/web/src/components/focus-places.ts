import { getBoot, screenOf } from '@/config/boot';
import type { ArchiveFocus, TaskOutFocus } from '@/stores/page-actions';
import { archiveEscapeTarget, rowAfter, rowNearby } from '@/utils/focus-targets';
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
