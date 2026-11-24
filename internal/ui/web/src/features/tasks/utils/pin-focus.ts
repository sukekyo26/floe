import type { Page } from '@/types/page';

// What of a pin's chip had the focus: its title (the link) or its ×
export type PinPart = 'pin' | 'pin-off';

export interface PinFocused {
  slug: string;
  on: PinPart;
}

// pinsSaid is what the row of pins says: drawn anew whole as soon as any of it changes, so that a screen reader says
// the chip with the focus again
export const pinsSaid = (pinned: readonly Page[]): string =>
  JSON.stringify(pinned.map((a) => [a.slug, a.title, Boolean(a.news), Boolean(a.folder)]));

// pinFocus is where the focus goes once the pins are drawn anew, of what had it: the same part of the same chip; of a
// chip gone, the title of the one in its place, not its ×, as a key held down would go on to take the next off; with
// none left, the help ('help')
export function pinFocus(was: readonly string[], now: readonly string[], focused: PinFocused): PinFocused | 'help' {
  if (now.includes(focused.slug)) return focused;
  // A chip not among those drawn before (none is, as the focus is read off them) goes to the first that is now
  const place = Math.max(was.indexOf(focused.slug), 0);
  const slug = now[Math.min(place, now.length - 1)];
  return slug === undefined ? 'help' : { slug, on: 'pin' };
}
