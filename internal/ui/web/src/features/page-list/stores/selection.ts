import { createStore, type Store } from '@/lib/store';
import { seenOnly } from '@/features/page-list/utils/bulk';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { inView } from '@/utils/list-view';

// The pages selected on the page list, by slug. rev counts the writes: what waits for a second press (the bulk delete)
// lets go as it changes. A write that changes nothing is not made, so that the list is not drawn again for it
export interface Selection {
  selected: Store<ReadonlySet<string>>;
  rev: Store<number>;
  toggle: (slug: string, on: boolean) => void;
  add: (slugs: readonly string[]) => void;
  clear: () => void;
  // keep lets go of every slug not in slugs
  keep: (slugs: ReadonlySet<string>) => void;
}

export function createSelection(): Selection {
  const selected = createStore<ReadonlySet<string>>(new Set());
  const rev = createStore(0);
  const write = (next: ReadonlySet<string>) => {
    selected.set(next);
    rev.set(rev.get() + 1);
  };
  return {
    selected,
    rev,
    toggle: (slug, on) => {
      const now = selected.get();
      if (now.has(slug) === on) return;
      const next = new Set(now);
      if (on) next.add(slug);
      else next.delete(slug);
      write(next);
    },
    add: (slugs) => {
      const now = selected.get();
      const fresh = slugs.filter((slug) => !now.has(slug));
      if (fresh.length > 0) write(new Set([...now, ...fresh]));
    },
    clear: () => {
      if (selected.get().size > 0) write(new Set());
    },
    keep: (slugs) => {
      const now = selected.get();
      if ([...now].every((slug) => slugs.has(slug))) return;
      write(new Set([...now].filter((slug) => slugs.has(slug))));
    },
  };
}

export const selection = createSelection();

// followList lets the selection go as the list's picks, view or search are given anew, not as a page of the view "new"
// is kept as seen; and, as the list is read, lets go of the pages its view no longer has. Started, it looks the
// selection over as a read does: the list put back after a draw that threw, the reads meanwhile unheard. It returns
// what stops it
export function followList(
  query: ListQuery = listQuery,
  pages: Pages = pagePages,
  sel: Selection = selection,
): () => void {
  let was = query.query.get();
  const onQuery = () => {
    const now = query.query.get();
    // Told again of what was seen already, as a write inside another's telling is told before the rest hear the first
    if (now === was) return;
    const only = seenOnly(was, now);
    was = now;
    if (!only) sel.clear();
  };
  const onPages = () => {
    const state = query.query.get();
    sel.keep(
      new Set(
        pages.state
          .get()
          .entries.filter((a) => inView(a, state, state.seen))
          .map((a) => a.slug),
      ),
    );
  };
  onPages();
  const stops = [query.query.subscribe(onQuery), pages.state.subscribe(onPages)];
  return () => {
    for (const stop of stops) stop();
  };
}
