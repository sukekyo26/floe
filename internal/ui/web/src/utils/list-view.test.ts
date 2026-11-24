import { describe, expect, it } from 'vitest';

import { arrange, inView, type ListState } from '@/utils/list-view';
import { projectOrder } from '@/utils/project-order';
import { searched } from '@/utils/search';
import type { Page } from '@/types/page';

const page = (slug: string, more: Partial<Page> = {}): Page => ({
  slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...more,
});
const ALL: ListState = { view: 'all', kind: '', template: '', project: '', tag: '', q: '' };

describe(arrange, () => {
  it('keeps the archived to their view, the new and those seen since to theirs, and narrows by the picks and words', () => {
    expect.assertions(4);
    const items = [
      page('a', { archived: true }),
      page('b', { news: 'new', project: 'p', tags: ['t'] }),
      page('c', { favorite: true, title: 'Find me' }),
    ];
    expect(arrange(items, ALL, new Set()).map((x) => x.slug)).toStrictEqual(['b', 'c']);
    expect(arrange(items, { ...ALL, view: 'new' }, new Set(['c'])).map((x) => x.slug)).toStrictEqual(['b', 'c']);
    expect(arrange(items, { ...ALL, project: 'p', tag: 't' }, new Set()).map((x) => x.slug)).toStrictEqual(['b']);
    expect(searched(' find ', items).map((x) => x.slug)).toStrictEqual(['c']);
  });

  it('puts no archived page in the view of the favourites', () => {
    expect.assertions(1);
    expect(inView(page('a', { archived: true, favorite: true }), { view: 'fav' }, new Set())).toBe(false);
  });
});

describe(projectOrder, () => {
  it('orders the projects by how many entries have them, then by name, and those of no entry after them', () => {
    expect.assertions(1);
    const items = [page('a', { project: 'y' }), page('b', { project: 'x' }), page('c', { project: 'y' })];
    expect([...projectOrder(items, ['z', '/', 'x'])]).toStrictEqual([
      ['y', 0],
      ['x', 1],
      ['z', 2],
    ]);
  });
});
