import { describe, expect, it } from 'vitest';

import { emptyOf } from '@/features/page-list/utils/list-empty';
import type { Page } from '@/types/page';
import { arrange, type ListState } from '@/utils/list-view';

const page = (slug: string, change: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...change,
});
const state = (change: Partial<ListState> = {}): ListState => ({
  view: 'all',
  kind: '',
  template: '',
  project: '',
  tag: '',
  q: '',
  ...change,
});
const of = (items: Page[], s: ListState, seen: ReadonlySet<string> = new Set(), read = true) =>
  emptyOf(items, s, seen, arrange(items, s, seen), read);

describe(emptyOf, () => {
  it('says which words stand for the rows, by the view and what the shelf and the search leave', () => {
    expect.assertions(1);
    const read = page('r');
    const gone = page('g', { archived: true });
    expect([
      of([], state()),
      of([gone], state()),
      of([read], state({ view: 'new' })),
      of([read], state({ view: 'fav' })),
      of([read], state({ view: 'archived' })),
      of([read], state({ q: 'nothing' })),
      of([read], state()),
    ]).toStrictEqual(['empty', 'all-archived', 'empty-new', 'empty-favorites', 'empty-archived', 'nomatch', null]);
  });

  it('says nothing before a list is taken', () => {
    expect.assertions(1);
    expect(of([], state(), new Set(), false)).toBeNull();
  });

  it('keeps the rows of the view "new" read since it was picked, without its empty words', () => {
    expect.assertions(1);
    expect(of([page('a')], state({ view: 'new' }), new Set(['a']))).toBeNull();
  });
});
