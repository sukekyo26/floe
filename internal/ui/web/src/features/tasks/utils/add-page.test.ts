import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { addHits, addSaid, hereBadge, pageFrom } from '@/features/tasks/utils/add-page';
import type { Page } from '@/types/page';

const page = (slug: string, change: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'p',
  ...change,
});
const to = { project: 'p', name: 'S1', category: 'design', rev: 3 };

describe(addHits, () => {
  it('offers the task project’s pages not archived and not on that category, by the words', () => {
    expect.assertions(2);
    const entries = [
      page('a'),
      page('b', { taskProject: 'q' }),
      page('c', { archived: true }),
      page('d', { task: { name: 'S1', category: 'design' } }),
      page('e', { task: { name: 'S1', category: 'build' } }),
      page('memo'),
    ];
    expect(addHits(entries, to, '').hits.map((a) => a.slug)).toStrictEqual(['a', 'e', 'memo']);
    expect(addHits(entries, to, 'mem').hits.map((a) => a.slug)).toStrictEqual(['memo']);
  });
});

describe(hereBadge, () => {
  it('says this task’s category, another task’s and category, or nothing', () => {
    expect.assertions(3);
    testBoot();
    expect(hereBadge(page('a', { task: { name: 'S1', category: 'build' } }), to)).toBe('ui_page_here_category:build');
    expect(hereBadge(page('a', { task: { name: 'S2', category: 'x' } }), to)).toBe('S2 · x');
    expect(hereBadge(page('a'), to)).toBe('');
  });
});

describe(addSaid, () => {
  it('says the task the write found the page on, or this task’s category the list had, else added', () => {
    expect.assertions(3);
    testBoot();
    const other = page('a', { task: { name: 'S1', category: 'build' } });
    expect(addSaid(other, to, pageFrom(other, to), undefined)).toBe('ui_page_moved:a');
    const fresh = page('b');
    expect(addSaid(fresh, to, pageFrom(fresh, to), { ...fresh, offTask: 'S9' })).toBe('ui_page_moved:b');
    expect(addSaid(fresh, to, pageFrom(fresh, to), undefined)).toBe('ui_page_added:b');
  });
});
