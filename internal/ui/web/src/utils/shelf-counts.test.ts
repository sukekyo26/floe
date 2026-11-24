import { describe, expect, it } from 'vitest';

import type { Page } from '@/types/page';
import { tally } from '@/utils/shelf-counts';

const page = (slug: string, tags: string[]): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  tags,
});
const tags = (a: Page) => a.tags ?? [];

describe(tally, () => {
  it('makes its rows of the list, keeps the one picked at 0, and counts what the rest leaves', () => {
    expect.assertions(2);
    const list = [page('a', ['x', 'y']), page('b', ['x']), page('c', [])];
    const t = tally(list, tags, 'gone', list.slice(0, 1));
    expect([...t.rows]).toStrictEqual([
      ['x', 2],
      ['y', 1],
      ['gone', 0],
    ]);
    expect(['x', 'y', 'gone'].map((v) => t.n(v))).toStrictEqual([1, 1, 0]);
  });

  it('counts the list itself where it is given as what is left (the task list)', () => {
    expect.assertions(1);
    const list = [page('a', ['x']), page('b', ['x'])];
    expect(tally(list, tags, '', list).n('x')).toBe(2);
  });
});
