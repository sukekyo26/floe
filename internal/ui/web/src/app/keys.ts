import type { KeyTable } from '@/hooks/use-keys';

export interface AppKeyDeps {
  // rows are the rows' keys: ↑ ↓ j k (useRowKeys)
  readonly rows: KeyTable;
  // taskPage: a task's page, where h goes up to the list
  readonly taskPage: boolean;
  readonly find: () => void;
  readonly findToggle: () => void;
  readonly help: () => void;
  readonly star: () => void;
  readonly diff: () => void;
  readonly up: () => void;
}

// appKeys is the screen's one table of keys (ui.html KEYS): the rows', / and Ctrl+K to the search, ? to the help's
// keys, f the row's star, d the diff of the page beside the list; on a task's page h too (ui.html 6807). No x: no row of
// the task screens has a box to tick
export const appKeys = ({ rows, taskPage, find, findToggle, help, star, diff, up }: AppKeyDeps): KeyTable => ({
  ...rows,
  '/': find,
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

// pressUp presses the way up of a task's page to the list (ui.html 6807)
export function pressUp(): void {
  document.getElementById('task-up')?.click();
}
