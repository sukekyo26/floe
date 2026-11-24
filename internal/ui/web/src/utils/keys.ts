// composing tells a key that goes to the input method: 229 as Safari sends its Enter that ends one without isComposing
export const composing = (e: Pick<KeyboardEvent, 'isComposing' | 'keyCode'>): boolean =>
  // oxlint-disable-next-line typescript/no-deprecated -- Safari tells its Enter that ends a composition by keyCode alone
  e.isComposing || e.keyCode === 229;

// What of a key event the keys below read
export interface KeyPress {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}

// keyName is the name a key goes by in the table of keys: Ctrl+K (⌘K too), else the key itself
export const keyName = (e: KeyPress): string =>
  (e.ctrlKey || e.metaKey) && !e.altKey && e.key === 'k' ? 'Ctrl+K' : e.key;

// What a key landed on, as far as the keys below look
export interface KeyTarget {
  closest: (selector: string) => unknown;
  matches: (selector: string) => boolean;
}

// The choices of versions, whose own keys go on (diffPickKey)
const VERSION_PICKS = '#diff select, [data-version-pick]';

// plainKey tells a key of the page's own: none with a modifier, with a modal open (seen as such, not by where the key
// lands: one held while it writes has the focus off its disabled fields) or typed into a field (a check box is none)
export const plainKey = (e: KeyPress & { target: KeyTarget | null }, dialogOpen: boolean): boolean =>
  !e.ctrlKey &&
  !e.metaKey &&
  !e.altKey &&
  !dialogOpen &&
  !e.target?.closest('input:not([type="checkbox"]), textarea, select, [contenteditable]');

// diffPickKey tells a key that goes on as anywhere from a choice of versions, whose items are found by letters that
// start with v: the focus stays there. The choice itself, closed: open, the key lands on its option, and is the list's
export const diffPickKey = (e: KeyPress & { target: KeyTarget | null }): boolean =>
  !e.ctrlKey &&
  !e.metaKey &&
  !e.altKey &&
  e.target?.matches(VERSION_PICKS) === true &&
  ['n', 'p', 'd', 'j', 'k'].includes(e.key);
