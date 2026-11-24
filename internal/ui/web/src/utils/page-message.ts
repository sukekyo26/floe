import type { ReadMark } from '@/types/page';

// Which iframe shows the page: whole (the frame), beside (the list's), or diff (the diff's)
export type FrameKind = 'whole' | 'beside' | 'diff';

// What a page's message asks of its frame, in the order it is done
export type PageAction =
  | { kind: 'key'; key: string }
  | { kind: 'open'; href: string }
  | { kind: 'changed' }
  | { kind: 'hash'; hash: string }
  | { kind: 'served'; mark: ReadMark };

// PAGE_KEYS are the keys a page may pass, those that move or open a modal alone (ui.md): none that writes
export const PAGE_KEYS: ReadonlySet<string> = new Set(['j', 'k', 'h', 'n', 'p', 'd', '?', '/', 'Ctrl+K']);

export interface MessageFrom {
  frame: FrameKind;
  // fromShown tells the message came from the window of the iframe shown
  fromShown: boolean;
  origin: string;
  // givenPath is the path this page gave the iframe, null for none
  givenPath: string | null;
  // focused tells the iframe has the focus, as a key pressed in the page is passed then alone
  focused: boolean;
  // keys are the screen's table of keys; a page passes those of PAGE_KEYS alone
  keys: ReadonlySet<string>;
  // here is this page's origin, to which a page may move the frame
  here: string;
}

// What a page may send, each of any value until checked
interface PageData {
  pageKey?: unknown;
  pageOpen?: unknown;
  pagePath?: unknown;
  pageChanged?: unknown;
  pageHash?: unknown;
  pageServed?: unknown;
}
const record = <T>(data: unknown): Partial<T> => (typeof data === 'object' && data !== null ? data : {});

// pageMessage is what the frame does of a page's message: of the iframe shown alone, and of origin null, as a page of
// floe is always sandboxed (any other is a site the iframe moved to). A key pressed in the page goes on while the
// iframe has the focus; a move goes to a page of floe alone; the rest only from the path the iframe was given, wherever
// a link took it. A move whose words are no URL throws the whole message away, the key with it
// LIMIT: origin null does not prove a page of floe: a site that sandboxes itself, or a data: URL, says it too. All it
// can do is what a page can, of PAGE_KEYS and a move to a whole page; a page's read stops at the path it must be at
export function pageMessage(data: unknown, from: MessageFrom): PageAction[] {
  if (!from.fromShown || from.origin !== 'null') return [];
  const d = record<PageData>(data);
  const actions: PageAction[] = [];
  if (typeof d.pageKey === 'string' && PAGE_KEYS.has(d.pageKey) && from.keys.has(d.pageKey) && from.focused) {
    actions.push({ kind: 'key', key: d.pageKey });
  }
  if (typeof d.pageOpen === 'string') {
    let to: URL;
    try {
      to = new URL(d.pageOpen, from.here);
    } catch {
      return [];
    }
    if (to.origin === from.here && /^\/(a\/[^/]+\/?)?$/.test(to.pathname) && !to.searchParams.has('raw')) {
      actions.push({ kind: 'open', href: to.href });
    }
  }
  if (d.pagePath !== from.givenPath) return actions;
  // A diff's page whose versions changed: what changed is read again, and nothing more of the message
  if (from.frame === 'diff' && d.pageChanged === true) return [...actions, { kind: 'changed' }];
  const hash = d.pageHash;
  if (from.frame === 'whole' && typeof hash === 'string' && (hash === '' || hash.startsWith('#'))) {
    actions.push({ kind: 'hash', hash });
  }
  const read = record<{ n: unknown; at: unknown }>(d.pageServed);
  if (Number.isInteger(read.n) && typeof read.at === 'string') {
    actions.push({ kind: 'served', mark: { n: read.n as number, at: read.at } });
  }
  return actions;
}
