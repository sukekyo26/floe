import { composing } from '@/utils/keys';

// The row reached, by slug as the rows are drawn anew, and by its place for when it is gone
export interface Reached {
  slug: string | null;
  at: number;
}

// FIRST is the search opened or its words changed: the first row
export const FIRST: Reached = { slug: null, at: 0 };

// findReached is the row reached among shown: the same page, or what has come to its place, or the last; -1 for none
export function findReached(shown: readonly { slug: string }[], reached: Reached): number {
  const same = shown.findIndex((a) => a.slug === reached.slug);
  return same === -1 ? Math.min(reached.at, shown.length - 1) : same;
}

// What a key does in the search modal: the arrows move over the rows from anywhere in it, but not while a menu is open
// in it, where Escape closes that menu alone; Enter on the field opens the row reached; Escape else closes the modal at
// once, words typed or not (the browser would first clear a search field's words); nothing while a word is being
// composed (an IME's Enter and arrows are its own)
export type FindKey = 'move' | 'open' | 'close' | 'closeMenu' | 'hold' | 'none';

export function findKey(
  e: { key: string; isComposing: boolean; keyCode: number },
  { onField, reached, menuOpen }: { onField: boolean; reached: number; menuOpen: boolean },
): FindKey {
  if (composing(e)) return 'none';
  const arrow = e.key === 'ArrowDown' || e.key === 'ArrowUp';
  if (menuOpen) return e.key === 'Escape' ? 'closeMenu' : arrow ? 'hold' : 'none';
  if (e.key === 'Escape') return 'close';
  if (arrow) return 'move';
  if (e.key === 'Enter' && onField && reached >= 0) return 'open';
  return 'none';
}
