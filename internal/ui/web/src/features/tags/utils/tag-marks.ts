import type { Page } from '@/types/page';

// A tag's mark in the tag modal: on all of its pages, on some, or on none
export type Mark = 'all' | 'some' | 'none';

export interface Marks {
  // on is how many of the pages each tag was on as the modal opened
  on: Map<string, number>;
  // start is the mark each chip opened with; want is the mark it has now
  start: Map<string, Mark>;
}

// tagNames is every tag of every page, in the reader's language's order
export const tagNames = (entries: Page[], lang: string): string[] =>
  [...new Set(entries.flatMap((a) => a.tags ?? []))].toSorted((x, y) => x.localeCompare(y, lang));

// startMarks are the marks of names over pages as the modal opens
export function startMarks(pages: Page[], names: string[]): Marks {
  const on = new Map(names.map((name) => [name, pages.filter((a) => a.tags?.includes(name)).length]));
  const start = new Map(
    [...on].map(([name, n]): [string, Mark] => [name, n === pages.length ? 'all' : n > 0 ? 'some' : 'none']),
  );
  return { on, start };
}

// nextMark is a chip pressed: all ⇄ none, and a chip that opened on some goes some → all → none → some, so that it can
// leave the pages as they are
export const nextMark = (now: Mark, start: Mark): Mark =>
  now === 'all' ? 'none' : now === 'none' && start === 'some' ? 'some' : 'all';

// changes are the tags to add (changed to all) and to remove (changed to none): only those changed, so that a tag written
// elsewhere meanwhile stays, and one left on some is left as each page has it
export function changes(start: Map<string, Mark>, want: Map<string, Mark>): { add: string[]; remove: string[] } {
  const to = (mark: Mark) => [...want].filter(([name, w]) => w === mark && start.get(name) !== mark).map(([n]) => n);
  return { add: to('all'), remove: to('none') };
}

// changed tells whether Apply can be pressed: a chip has a mark other than its start
export const changed = (start: Map<string, Mark>, want: Map<string, Mark>): boolean =>
  [...want].some(([name, w]) => w !== start.get(name));

// toSend are the slugs still listed whose tags these would change: a page gone, or one already as asked, is not sent
export function toSend(entries: Page[], slugs: string[], add: string[], remove: string[]): string[] {
  return slugs.filter((slug) => {
    const a = entries.find((x) => x.slug === slug);
    const now = a?.tags ?? [];
    return a !== undefined && (add.some((name) => !now.includes(name)) || remove.some((name) => now.includes(name)));
  });
}

const same = (x: string, y: string) => x.toLowerCase() === y.toLowerCase();

// chips are the names drawn: those on its pages first, then by name, and of them those that hold find, as the find
// field matches, whatever the case
export function chips(want: Map<string, Mark>, on: Map<string, number>, find: string, lang: string): string[] {
  const words = find.trim().toLowerCase();
  return [...want.keys()]
    .toSorted((x, y) => Number(!on.get(x)) - Number(!on.get(y)) || x.localeCompare(y, lang))
    .filter((name) => name.toLowerCase().includes(words));
}

// fresh tells whether find can be made a tag: words that no tag has, whatever the case (S426: a tag that differs from
// one only in case is not made from the screen)
export const fresh = (want: Map<string, Mark>, find: string): boolean => {
  const words = find.trim();
  return words !== '' && ![...want.keys()].some((name) => same(name, words));
};

// taken is the tag Enter takes for find: the one of the same name whatever the case (the one written as typed first,
// else the first of names, the chips as drawn); else the one chip left; else none, and Enter makes a tag (S426)
export function taken(names: string[], find: string): string | null {
  const words = find.trim();
  if (!words) return null;
  const alike = names.filter((name) => same(name, words));
  return alike.find((name) => name === words) ?? alike[0] ?? (names.length === 1 ? (names[0] ?? null) : null);
}
