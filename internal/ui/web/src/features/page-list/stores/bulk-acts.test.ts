import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import type { Boot } from '@/config/boot';
import { bulkActs } from '@/features/page-list/stores/bulk-acts';
import { createSelection } from '@/features/page-list/stores/selection';
import { createStore } from '@/lib/store';
import type { ActOn } from '@/stores/act-on';
import { createListQuery } from '@/stores/list-query';
import type { PageWrites } from '@/stores/page-writes';
import type { Pages, PagesState } from '@/stores/pages';
import type { Page } from '@/types/page';

const page = (slug: string, change: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...change,
});
const made = (search: string, entries: Page[], refused: string[] = []) => {
  testBoot();
  const query = createListQuery({
    boot: () => ({ screen: 'list', states: [] }) as unknown as Boot,
    search: () => search,
    replace: () => undefined,
    folders: { takeOpen: () => undefined },
  });
  const pages = { state: createStore({ read: true, entries } as PagesState) } as unknown as Pages;
  const selection = createSelection();
  const sent: unknown[] = [];
  const said: string[] = [];
  const actOn: ActOn = async (act, words, slugs, settled) => {
    const reasons = await Promise.all(slugs.map(act));
    settled?.(reasons);
    said.push(words.done(slugs.length));
  };
  const writes = {
    patchPage: (slug: string, body: unknown) => {
      sent.push([slug, body]);
      return Promise.resolve(refused.includes(slug) ? 'no' : '');
    },
    deletePage: (slug: string) => {
      sent.push([slug, 'delete']);
      return Promise.resolve(refused.includes(slug) ? 'no' : '');
    },
  } as unknown as PageWrites;
  return { selection, sent, said, acts: bulkActs({ selection, actOn, writes, pages, query }) };
};

describe(bulkActs, () => {
  it('selects every entry the list draws, of its view and search', () => {
    expect.assertions(1);
    const { selection, acts } = made('?q=a', [
      page('a'),
      page('ab', { folder: true }),
      page('b'),
      page('ax', { archived: true }),
    ]);
    acts.selectAll();
    expect([...selection.selected.get()]).toStrictEqual(['a', 'ab']);
  });

  it('archives the selected, restores them on the view of the archived, and keeps those refused selected', async () => {
    expect.assertions(3);
    const { selection, sent, acts } = made('', [page('a'), page('b')], ['b']);
    selection.add(['a', 'b']);
    await acts.archive();
    expect(sent).toStrictEqual([
      ['a', { archived: true }],
      ['b', { archived: true }],
    ]);
    expect([...selection.selected.get()]).toStrictEqual(['b']);
    const restore = made('?view=archived', [page('c', { archived: true })]);
    restore.selection.add(['c']);
    await restore.acts.archive();
    expect(restore.sent).toStrictEqual([['c', { archived: false }]]);
  });

  it('says it unlinked the links, in the words of before the write, and tags the selected leaving out the rest', async () => {
    expect.assertions(2);
    const { selection, said, acts } = made('', [page('l', { path: '/x.html' })]);
    selection.add(['l']);
    await acts.remove();
    expect(said).toStrictEqual(['ui_unlink_bulk_done:1']);
    selection.add(['l', 'm']);
    acts.tags().after(new Set(['m']));
    expect([...selection.selected.get()]).toStrictEqual(['m']);
  });
});
