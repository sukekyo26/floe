import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { ListCount, ListWhat } from '@/features/page-list/components/list-head';
import { ListShelf } from '@/features/page-list/components/list-shelf';
import { News } from '@/features/page-list/components/news';
import { newsTaken } from '@/features/page-list/utils/list-shelf';
import { createStore } from '@/lib/store';
import type { ColumnsStore } from '@/stores/columns';
import type { ListQuery, Query } from '@/stores/list-query';
import type { Pages } from '@/stores/pages';
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
const stores = (entries: Page[], change: Partial<Query> = {}, open = true) => ({
  query: {
    query: createStore<Query>({
      view: 'all',
      kind: '',
      template: '',
      project: '',
      tag: '',
      q: '',
      seen: new Set(),
      ...change,
    }),
  } as unknown as ListQuery,
  pages: { state: createStore({ entries }) } as unknown as Pages,
  columns: { columns: createStore({ open }) } as unknown as ColumnsStore,
});
const none = () => undefined;
const tag = (html: string, start: string) => String(new RegExp(`${start}[^>]*>`).exec(html)?.[0]);

describe(ListShelf, () => {
  it('hides the kinds with one to choose, and the add button where there is no page at all', () => {
    expect.assertions(3);
    testBoot();
    const one = renderToString(<ListShelf onAdd={none} {...stores([page('a')])} />);
    expect(tag(one, '<div id="shelf-kinds"')).toContain('hidden=""');
    expect(tag(one, '<div id="shelf-add"')).not.toContain('hidden=""');
    expect(tag(renderToString(<ListShelf onAdd={none} {...stores([])} />), '<div id="shelf-add"')).toContain(
      'hidden=""',
    );
  });

  it('keeps a project picked whose pages are gone, at 0, and counts what the rest of the shelf leaves', () => {
    expect.assertions(2);
    testBoot();
    const items = [page('a', { project: 'p', tags: ['x'] }), page('b', { project: 'q' })];
    const html = renderToString(<ListShelf onAdd={none} {...stores(items, { tag: 'x', project: 'gone' })} />);
    expect(html).toMatch(/data-project="gone"[^>]*>.*?data-testid="n"[^>]*>0</);
    expect(html).toMatch(
      /data-project="p"[^>]*>.*?data-testid="n"[^>]*>1<.*data-project="q"[^>]*>.*?data-testid="n"[^>]*>0</,
    );
  });
});

describe(News, () => {
  it('counts every unread page not archived, apart from the view New, and is gone at none', () => {
    expect.assertions(5);
    testBoot();
    const items = [
      page('a', { news: 'new', tags: ['x'] }),
      page('b', { news: 'new' }),
      page('c', { news: 'new', archived: true }),
    ];
    const some = stores(items, { tag: 'x' });
    expect(renderToString(<News {...some} />)).toMatch(
      /id="news" [^>]*href="\/\?view=new"[^>]*>.*id="news-n">ui_news:2</,
    );
    // The head counts the new ones the picks leave
    expect(renderToString(<ListCount {...some} />)).toContain('ui_list_head:1');
    expect(tag(renderToString(<News {...stores([page('a')])} />), '<a')).toContain('hidden=""');
    // From 70rem the shelf's views say it
    expect(tag(renderToString(<News {...some} />), '<a')).toContain('wide:hidden');
    // A link of words, not a button (the user's choice, S422-P8)
    expect(tag(renderToString(<News {...some} />), '<a')).not.toMatch(/\bborder\b|rounded-small/);
  });

  it('narrows the list in place on the page list alone, a link to it on the other screens', () => {
    expect.assertions(1);
    expect((['list', 'tasks', 'frame'] as const).map((screen) => newsTaken(screen))).toStrictEqual([
      true,
      false,
      false,
    ]);
  });
});

describe(ListWhat, () => {
  it('says the view and the picks with the shelf folded, and is hidden with it open or nothing to say', () => {
    expect.assertions(3);
    testBoot();
    const folded = renderToString(<ListWhat onShow={none} {...stores([], { view: 'fav', tag: 'x' }, false)} />);
    expect(folded).toMatch(/id="shelf-what"(?![^>]*hidden="")[^>]*>ui_view_favorites:undefined · ui_narrow_n:1</);
    expect(tag(renderToString(<ListWhat onShow={none} {...stores([], { view: 'fav' }, true)} />), '<button')).toContain(
      'hidden=""',
    );
    expect(tag(renderToString(<ListWhat onShow={none} {...stores([], {}, false)} />), '<button')).toContain(
      'hidden=""',
    );
  });
});
