import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { setBoot, type Boot } from '@/config/boot';
import { addHits, addSaid, groupName, hereBadge, pageFrom } from '@/features/tasks/utils/add-page';
import type { PageWritten } from '@/lib/tasks';
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

// No category, as Add a page puts a page there (S438)
const none = { ...to, category: '' };

describe('no category', () => {
  it('offers this task’s pages of a category and other tasks’, not those of no category', () => {
    expect.assertions(1);
    const entries = [
      page('n', { task: { name: 'S1' } }),
      page('d', { task: { name: 'S1', category: 'design' } }),
      page('o', { task: { name: 'S9', category: '案' } }),
      page('free'),
    ];
    expect(addHits(entries, none, '').hits.map((a) => a.slug)).toStrictEqual(['d', 'o', 'free']);
  });

  it('names no category in words of its own', () => {
    expect.assertions(2);
    testBoot();
    expect(groupName('')).toBe('ui_category_none:undefined');
    expect(groupName('design')).toBe('design');
  });

  it('says where the page was put from, as the person reads it', () => {
    expect.assertions(4);
    setBoot({
      messages: {
        ui_page_added_no_category: '「%s」をカテゴリなしで付けました',
        ui_page_moved_no_category: '「%s」を%sから外し、カテゴリなしにしました',
        ui_page_from_task_category: '%s の「%s」',
        ui_page_from_category: '「%s」',
      },
    } as unknown as Boot);
    const said = (a: Page, off?: PageWritten) => addSaid(a, none, pageFrom(a, none), off);
    const memo = page('メモ');
    expect(said(memo)).toBe('「メモ」をカテゴリなしで付けました');
    expect(said(memo, { ...memo, offTask: '#9', offCategory: '案' })).toBe(
      '「メモ」を#9 の「案」から外し、カテゴリなしにしました',
    );
    expect(said(memo, { ...memo, offTask: '#9' })).toBe('「メモ」を#9から外し、カテゴリなしにしました');
    expect(said(page('メモ', { task: { name: 'S1', category: 'design' } }))).toBe(
      '「メモ」を「design」から外し、カテゴリなしにしました',
    );
  });
});
