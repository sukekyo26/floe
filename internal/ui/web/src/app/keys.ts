import type { KeyTable } from '@/hooks/use-keys';

export interface AppKeyDeps {
  // rows are the rows' keys: ↑ ↓ j k (useRowKeys)
  readonly rows: KeyTable;
  // taskPage: a task's page, where h goes up to the list
  readonly taskPage: boolean;
  readonly find: () => void;
  // search is where / goes on a screen with the list's search in its bar (ui.html 6488); left out, / opens find
  readonly search?: (() => void) | undefined;
  readonly findToggle: () => void;
  readonly help: () => void;
  readonly star: () => void;
  readonly diff: () => void;
  readonly up: () => void;
}

// appKeys is the screen's one table of keys (ui.html KEYS): the rows' (x among them where the rows have boxes), / to the
// list's search or the search modal, Ctrl+K to the search modal, ? to the help's keys, f the row's star, d the diff of
// the page beside the list; on a task's page h too (ui.html 6807)
export const appKeys = ({ rows, taskPage, find, search, findToggle, help, star, diff, up }: AppKeyDeps): KeyTable => ({
  ...rows,
  '/': search ?? find,
  '?': help,
  'Ctrl+K': findToggle,
  f: star,
  d: diff,
  ...(taskPage ? { h: up } : {}),
});

const rowWithFocus = (): Element | undefined =>
  [...document.querySelectorAll('#list > li[data-key]')].find((li) => li.contains(document.activeElement));

// pressStar presses the star of the row with the focus (ui.html 6934)
export function pressStar(): void {
  const star = rowWithFocus()?.querySelector('[data-testid=star]:not([hidden])');
  if (star instanceof HTMLElement) star.click();
}

// pressDiff presses the diff's button of the bar over the page beside the list, where it shows (ui.html 6935)
export function pressDiff(): void {
  const open = document.querySelector('#read-actions [data-testid=diff-open]:not([hidden])');
  if (open instanceof HTMLElement) open.click();
}

// pressUp presses the way up of a task's page to the list: the bar's task list (ui.html 6807)
export function pressUp(): void {
  document.getElementById('to-tasks')?.click();
}
