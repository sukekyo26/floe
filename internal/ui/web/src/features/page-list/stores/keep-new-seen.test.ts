import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { keepNewSeen } from '@/features/page-list/stores/keep-new-seen';
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

describe(keepNewSeen, () => {
  it('keeps the unread seen on the view "new", again after a pick that lets them go with the view unchanged', () => {
    expect.assertions(3);
    const query = createListQuery({
      boot: () => ({ screen: 'list', states: [] }) as unknown as Boot,
      search: () => '?view=new',
      replace: () => undefined,
      folders: { takeOpen: () => undefined },
    });
    const state = createStore({ read: true, entries: [page('a', 'new'), page('b')] } as PagesState);
    const stop = keepNewSeen(query, { state } as unknown as Pages);
    expect([...query.query.get().seen]).toStrictEqual(['a']);
    // The bar's new pages: the view stays "new", the seen go and come back
    query.filter({ view: 'new' });
    expect([...query.query.get().seen]).toStrictEqual(['a']);
    stop();
    query.filter({ view: 'new' });
    expect(query.query.get().seen.size).toBe(0);
  });
});
