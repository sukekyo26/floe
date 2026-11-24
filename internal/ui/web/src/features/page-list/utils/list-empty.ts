import type { Page } from '@/types/page';
import { inView, type ListState } from '@/utils/list-view';

// What the page list says in place of its rows, by the id of its words
export type Empty = 'empty' | 'all-archived' | 'empty-new' | 'empty-favorites' | 'empty-archived' | 'nomatch';

const OF_VIEW: Partial<Record<string, Empty>> = {
  new: 'empty-new',
  fav: 'empty-favorites',
  archived: 'empty-archived',
};

// emptyOf is what the list says with no row to draw: no page at all (how to add one, as listEmpty says), all of them
// archived (the way to them), none of the view, or none the shelf and the search leave. shown is what they leave; read,
// whether a list was taken yet: till then, it says nothing (a read that failed has no list)
export function emptyOf(
  items: readonly Page[],
  state: ListState,
  seen: ReadonlySet<string>,
  shown: readonly Page[],
  read: boolean,
): Empty | null {
  if (!read) return null;
  const empty = !items.some((a) => inView(a, state, seen));
  if (!empty) return shown.length === 0 ? 'nomatch' : null;
  if (state.view === 'all') return items.some((a) => a.archived) ? 'all-archived' : 'empty';
  return OF_VIEW[state.view] ?? null;
}
