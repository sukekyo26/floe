import type { KeyTable } from '@/hooks/use-keys';

export interface AppKeyDeps {
  // rows are the rows' keys: ↑ ↓ j k (useRowKeys); a page whole's j k to the next pages, and a diff's n p
  readonly rows: KeyTable;
  readonly find: () => void;
  // search is where / goes on a screen with the list's search in its bar (ui.html 6488); left out, / opens find
  readonly search?: (() => void) | undefined;
  readonly findToggle: () => void;
  readonly help: () => void;
  readonly star: () => void;
  readonly diff: () => void;
  // up is where h goes: a task's page up to the list, a page whole back to where it was opened from; none elsewhere
  readonly up?: (() => void) | undefined;
}

// appKeys is the screen's one table of keys (ui.html KEYS): the rows' (x among them where the rows have boxes), / to the
// list's search or the search modal, Ctrl+K to the search modal, ? to the help's keys, f the row's star, d the diff of
// the page open; on a task's page and a page whole h too (ui.html 6807, 6757)
export const appKeys = ({ rows, find, search, findToggle, help, star, diff, up }: AppKeyDeps): KeyTable => ({
  ...rows,
  '/': search ?? find,
  '?': help,
  'Ctrl+K': findToggle,
  f: star,
  d: diff,
  ...(up ? { h: up } : {}),
});

// frameRows are a page whole's keys in place of the rows': j and k to the next and the one before of the pages it goes
// through, and in a diff n and p to its next and previous change (ui.html 6756–6763). No ↑ ↓ or x: it has no rows
export const frameRows = ({
  move,
  diff,
}: {
  readonly move: (step: 1 | -1) => void;
  readonly diff: ((step: 1 | -1) => void) | null;
}): KeyTable => ({
  j: () => {
    move(1);
  },
  k: () => {
    move(-1);
  },
  ...(diff
    ? {
        n: () => {
          diff(1);
        },
        p: () => {
          diff(-1);
        },
      }
    : {}),
});

const rowWithFocus = (): Element | undefined =>
  [...document.querySelectorAll('#list > li[data-key]')].find((li) => li.contains(document.activeElement));

// pressStar presses the star in box where it shows: the row with the focus, or a page whole's bar (ui.html 6934, 6759)
export function pressStar(box: Element | null | undefined = rowWithFocus()): void {
  const star = box?.querySelector('[data-testid=star]:not([hidden])');
  if (star instanceof HTMLElement) star.click();
}

// pressDiff presses the diff's button in box where it shows: the bar over the page beside the list, or a page whole's
// (ui.html 6935, 6760)
export function pressDiff(box: Element | null = document.getElementById('read-actions')): void {
  const open = box?.querySelector('[data-testid=diff-open]:not([hidden])');
  if (open instanceof HTMLElement) open.click();
}

// pressUp presses the link h goes by, so that the key and a press go one way: a task's page's task list in the bar
// (ui.html 6807), a page whole's way back (6757)
export function pressUp(id: string): void {
  document.getElementById(id)?.click();
}
