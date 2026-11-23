import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { keepNewSeen } from '@/features/page-list/stores/keep-new-seen';
import { createSelection, followList } from '@/features/page-list/stores/selection';
import { createStore } from '@/lib/store';
import { createListQuery } from '@/stores/list-query';
import type { Pages, PagesState } from '@/stores/pages';
import type { Page } from '@/types/page';

const page = (slug: string, news?: string): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...(news ? { news } : {}),
});
const made = (search: string, entries: Page[]) => {
  const query = createListQuery({
    boot: () => ({ screen: 'list', states: [] }) as unknown as Boot,
    search: () => search,
    replace: () => undefined,
    folders: { takeOpen: () => undefined },
  });
  const state = createStore({ read: true, entries } as PagesState);
  const pages = { state } as unknown as Pages;
  return { query, state, pages, sel: createSelection() };
};

describe(createSelection, () => {
  it('writes, and counts a write, only what changes the selection', () => {
    expect.assertions(4);
    const sel = createSelection();
    sel.toggle('a', true);
    sel.toggle('a', true);
    sel.add(['a', 'b']);
    sel.add(['b']);
    expect([[...sel.selected.get()], sel.rev.get()]).toStrictEqual([['a', 'b'], 2]);
    sel.keep(new Set(['a', 'b']));
    sel.keep(new Set(['a']));
    expect([[...sel.selected.get()], sel.rev.get()]).toStrictEqual([['a'], 3]);
    sel.clear();
    sel.clear();
    expect(sel.rev.get()).toBe(4);
    sel.toggle('a', false);
    expect(sel.rev.get()).toBe(4);
  });
});

describe(followList, () => {
  it('lets the selection go as the picks or the search are given, the same values too, and not as a page is seen', () => {
    expect.assertions(4);
    const { query, pages, sel } = made('', [page('a'), page('b')]);
    followList(query, pages, sel);
    sel.add(['a']);
    query.filter({ kind: '' });
    expect(sel.selected.get().size).toBe(0);
    sel.add(['a']);
    query.setWords('');
    expect(sel.selected.get().size).toBe(0);
    sel.add(['a']);
    query.see(['b']);
    expect([...sel.selected.get()]).toStrictEqual(['a']);
    query.filter({ view: 'fav' });
    expect(sel.selected.get().size).toBe(0);
  });

  // The bar's new pages, with nothing picked: the same values, the seen let go and kept again by keepNewSeen
  const barNews = (keepsFirst: boolean) => {
    const { query, pages, sel } = made('?view=new', [page('a', 'new'), page('b')]);
    const listen = keepsFirst
      ? [() => keepNewSeen(query, pages), () => followList(query, pages, sel)]
      : [() => followList(query, pages, sel), () => keepNewSeen(query, pages)];
    for (const start of listen) start();
    sel.add(['a']);
    query.filter({ view: 'new', kind: '', template: '', project: '', tag: '' });
    return sel.selected.get().size;
  };

  it('lets it go on the bar’s new pages, whichever of it and keepNewSeen hears first, the seen as they were too', () => {
    expect.assertions(1);
    expect([barNews(true), barNews(false)]).toStrictEqual([0, 0]);
  });

  it('looks the selection over as it starts, as the list put back after a draw that threw has missed the reads', () => {
    expect.assertions(1);
    const { query, state, pages, sel } = made('', [page('a'), page('b')]);
    sel.add(['a', 'b']);
    state.set({ ...state.get(), entries: [page('a')] });
    followList(query, pages, sel);
    expect([...sel.selected.get()]).toStrictEqual(['a']);
  });

  it('lets go of the pages the view no longer has as the list is read, and keeps those it has', () => {
    expect.assertions(1);
    const { query, state, pages, sel } = made('', [page('a'), page('b')]);
    followList(query, pages, sel);
    sel.add(['a', 'b']);
    state.set({ ...state.get(), entries: [page('a')] });
    expect([...sel.selected.get()]).toStrictEqual(['a']);
  });
});
