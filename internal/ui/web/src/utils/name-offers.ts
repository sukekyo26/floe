import { nextReached } from '@/utils/list-keys';

// A name offered under a field: what it puts in, what it shows, and words after it
export interface Offer {
  name: string;
  label: string;
  note?: string;
}

export interface OfferWays {
  // detail is how a name is shown (the name alone when left out)
  detail?: (name: string) => Offer;
  // fresh is what a name of none offers, after the rest
  fresh?: (name: string) => Offer;
}

// nameOffers is what a field of one name offers, as written: the names that hold it, those that start with it first; a
// name written whole offers them all, to change to, unless it was just typed whole (typed) and none goes on from it,
// which is done with: nothing then covers what is under the field
export function nameOffers(
  names: readonly string[],
  written: string,
  typed: boolean,
  { detail = (name) => ({ name, label: name }), fresh }: OfferWays = {},
): Offer[] {
  const word = written.trim();
  const lower = word.toLowerCase();
  const whole = names.some((name) => name.toLowerCase() === lower);
  const goesOn = names.some((name) => name.length > lower.length && name.toLowerCase().startsWith(lower));
  if (typed && whole && !goesOn) return [];
  const hits = whole ? [...names] : names.filter((name) => name.toLowerCase().includes(lower));
  const first = hits.filter((name) => name.toLowerCase().startsWith(whole ? '' : lower));
  const rest = hits.filter((name) => !first.includes(name));
  return [...[...first, ...rest].map(detail), ...(fresh && word && !whole ? [fresh(word)] : [])];
}

// What a key on a field of names does: ↓ ↑ open the names, or reach the next of them; Enter puts in the one reached;
// Escape closes them, and goes no further (not to the modal); Tab closes them as the focus goes on
export type SuggestKey = { act: 'open' } | { act: 'reach'; at: number } | { act: 'put'; at: number } | { act: 'close' };

export function suggestKey(key: string, shown: boolean, reached: number, n: number): SuggestKey | null {
  if (key === 'ArrowDown' || key === 'ArrowUp') {
    if (!shown) return { act: 'open' };
    return n > 0 ? { act: 'reach', at: nextReached(reached, n, key) } : null;
  }
  if (key === 'Enter' && shown && reached >= 0) return { act: 'put', at: reached };
  if (key === 'Escape' && shown) return { act: 'close' };
  if (key === 'Tab') return { act: 'close' };
  return null;
}
