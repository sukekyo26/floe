import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { syncList } from '@/features/page-list/stores/sync-list';
import type { Kept } from '@/lib/storage';
import { createStore } from '@/lib/store';
import { createListQuery } from '@/stores/list-query';
import type { Pages, PagesState } from '@/stores/pages';

const boot = () => ({ screen: 'list', lang: 'en', states: [] }) as unknown as Boot;
const none: Kept = { read: () => null, write: () => undefined, remove: () => undefined };

function made() {
  const urls: string[] = [];
  const query = createListQuery({
    boot,
    search: () => '',
    storage: none,
    replace: (url) => urls.push(url),
    href: () => 'http://x/',
    folders: { takeOpen: () => undefined },
  });
  const state = createStore<PagesState>({
    read: false,
    entries: [],
    tasks: { error: '', tasks: {}, raw: '' },
    readWhy: '',
    drawWhy: '',
  });
  return { urls, query, state, pages: { state } as unknown as Pages };
}

describe(syncList, () => {
  it('writes the URL as it starts, as the list is narrowed before any read and as it is read, and stops', () => {
    expect.assertions(4);
    const { urls, query, state, pages } = made();
    const stop = syncList(query, pages);
    expect(urls).toStrictEqual(['http://x/']);
    query.filter({ kind: 'link' });
    expect(urls.at(-1)).toBe('http://x/?kind=link');
    state.set({ ...state.get(), read: true });
    expect(urls).toHaveLength(3);
    stop();
    query.filter({ kind: 'page' });
    expect(urls).toHaveLength(3);
  });

  it('writes no URL as the pages change while none was read, a failed read', () => {
    expect.assertions(1);
    const { urls, query, state, pages } = made();
    syncList(query, pages);
    state.set({ ...state.get(), readWhy: 'failed' });
    expect(urls).toHaveLength(1);
  });
});
