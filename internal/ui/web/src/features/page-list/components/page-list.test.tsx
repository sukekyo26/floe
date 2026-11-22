import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import type { Openers } from '@/components/page-menu';
import { testBoot } from '@/components/test-boot';
import { ListEmpty } from '@/features/page-list/components/list-empty';
import { ListRow } from '@/features/page-list/components/list-row';
import { PageList } from '@/features/page-list/components/page-list';
import { createStore } from '@/lib/store';
import type { ListQuery, Query } from '@/stores/list-query';
import type { Folders } from '@/stores/open-folders';
import type { OpenAt, OpenPage } from '@/stores/open-page';
import type { Pages } from '@/stores/pages';
import type { Listing, Page } from '@/types/page';

const page = (slug: string, change: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: new Date(2026, 10, 21, 9).toISOString(),
  taskProject: 'proj',
  ...change,
});
const none = () => undefined;
const openers = { onTags: none, onTheme: none, onEdit: none, onTaskSet: none } as Openers;
const tagOf = (html: string, start: string) => String(new RegExp(`${start}[^>]*>`).exec(html)?.[0]);

const drawList = (
  entries: Page[],
  {
    read = true,
    opened = [] as string[],
    listings = new Map<string, Listing | Error>(),
    picked = null as OpenAt | null,
  } = {},
) => {
  testBoot();
  // The day's headings from this time, the rows' times of the same day
  vi.useFakeTimers({ now: new Date(2026, 10, 21, 12) });
  const html = renderToString(
    <PageList
      openers={openers}
      onAdd={none}
      query={
        {
          query: createStore<Query>({
            view: 'all',
            kind: '',
            template: '',
            project: '',
            tag: '',
            q: '',
            seen: new Set(),
          }),
        } as unknown as ListQuery
      }
      pages={{ state: createStore({ read, entries }) } as unknown as Pages}
      folders={{ list: createStore({ opened: new Set(opened), listings }) } as unknown as Folders}
      openPage={{ picked: createStore(picked) } as unknown as OpenPage}
    />,
  );
  vi.useRealTimers();
  return html;
};

describe(PageList, () => {
  it('draws the rows under their day’s heading, a folder open followed by its tree, its page picked marked', () => {
    expect.assertions(2);
    const listings = new Map<string, Listing | Error>([
      ['f', { dirs: ['d'], files: [{ file: 'x.html', updatedAt: '' }], more: 0 }],
      ['f:d', { dirs: [], files: [{ file: 'd/y.html', updatedAt: '' }], more: 0 }],
    ]);
    const html = drawList([page('a'), page('f', { folder: true, path: '/p' })], {
      opened: ['f', 'f:d'],
      listings,
      picked: { slug: 'f', file: 'x.html', v: 0 },
    });
    expect([...html.matchAll(/<li[^>]*? data-key="([^"]+)"/g)].map((m) => m[1])).toStrictEqual([
      'group:ui_day_today:undefined',
      'entry:a',
      'entry:f',
      'dir:f:d',
      'page:f:d/y.html',
      'page:f:x.html',
    ]);
    expect(tagOf(html, '<li data-testid="tree" data-key="page:f:x.html"')).toContain('aria-current="true"');
  });

  it('says nothing in place of rows before the first read, and that there is no page after it', () => {
    expect.assertions(2);
    const shown = (html: string) =>
      [...html.matchAll(/<div id="([^"]+)" data-testid="state"(?! hidden)/g)].map((m) => m[1]);
    expect(shown(drawList([], { read: false }))).toStrictEqual([]);
    expect(shown(drawList([]))).toStrictEqual(['empty']);
  });
});

describe(ListRow, () => {
  const row = (a: Partial<Page>) =>
    renderToString(
      <ListRow
        page={page('a', a)}
        index={0}
        ranks={new Map()}
        openers={openers}
        picked={false}
        unread={false}
        kept={false}
        folderOpen={false}
        onTag={none}
        onFolder={none}
        presses={{ press: none, key: none }}
      />,
    );

  it('links to the task the page is on and the task list it is pinned on, and makes each tag a button', () => {
    expect.assertions(4);
    testBoot();
    const html = row({ task: { name: '#1', pin: true }, tags: ['x'] });
    expect([...html.matchAll(/data-testid="task-tag" href="([^"]+)"/g)].map((m) => m[1])).toStrictEqual([
      '/tasks?project=proj&amp;task=%231',
      '/tasks?project=proj',
    ]);
    expect(tagOf(html, '<button type="button" data-testid="tagpill"')).toContain('aria-label="ui_tag_filter_named:x"');
    expect(row({ task: { pin: true }, archived: true })).not.toContain('task-tag');
    expect(tagOf(row({ folder: true }), '<li')).toContain('data-kind="folder"');
  });
});

describe(ListEmpty, () => {
  it('draws all six words, the one shown alone without hidden', () => {
    expect.assertions(2);
    testBoot();
    const html = renderToString(
      <ListEmpty shown="nomatch" archived={2} onAdd={none} onShowArchived={none} onClear={none} />,
    );
    expect(
      [...html.matchAll(/<div id="([^"]+)" data-testid="state"( hidden="")?/g)].map((m) => [m[1], m[2]]),
    ).toStrictEqual([
      ['empty', ' hidden=""'],
      ['all-archived', ' hidden=""'],
      ['empty-new', ' hidden=""'],
      ['empty-favorites', ' hidden=""'],
      ['empty-archived', ' hidden=""'],
      ['nomatch', undefined],
    ]);
    expect(html).toMatch(/id="clear"/);
  });
});
