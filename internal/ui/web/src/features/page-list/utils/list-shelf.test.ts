import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import {
  chipPress,
  listEmpty,
  narrowWords,
  newCount,
  newsPress,
  tagPress,
  unseenSlugs,
  viewCounts,
  viewLit,
  viewPress,
} from '@/features/page-list/utils/list-shelf';
import type { Page } from '@/types/page';
import type { ListState } from '@/utils/list-view';

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
const none = new Set<string>();

describe(viewCounts, () => {
  it('counts each view with the picks and the search, All with the search alone, New of those new now', () => {
    expect.assertions(1);
    const items = [
      page('a', { news: 'new', tags: ['x'] }),
      page('b', { favorite: true, tags: ['x'] }),
      page('c', { tags: ['y'] }),
      page('d', { archived: true, tags: ['x'] }),
    ];
    // b was read on the view New, and stays on it
    expect(viewCounts(items, state({ tag: 'x', view: 'new' }), new Set(['b']))).toStrictEqual({
      all: 3,
      new: 1,
      fav: 1,
      archived: 1,
    });
  });
});

describe(viewPress, () => {
  it('lets a lit view go to All, lets the view and the picks go from an unlit All, and leaves a lit All', () => {
    expect.assertions(2);
    expect([
      viewPress(state({ view: 'fav' }), 'fav'),
      viewPress(state(), 'new'),
      viewPress(state({ view: 'new', tag: 'x' }), 'all'),
      viewPress(state({ tag: 'x' }), 'all'),
      viewPress(state({ q: 'words' }), 'all'),
    ]).toStrictEqual([
      { view: 'all' },
      { view: 'new' },
      { view: 'all', kind: '', template: '', project: '', tag: '' },
      { view: 'all', kind: '', template: '', project: '', tag: '' },
      null,
    ]);
    expect([viewLit(state({ tag: 'x' }), 'all'), viewLit(state({ q: 'w' }), 'all')]).toStrictEqual([false, true]);
  });
});

describe(chipPress, () => {
  it('picks a chip, and lets the one picked go', () => {
    expect.assertions(1);
    expect([chipPress(state(), 'tag', 'x'), chipPress(state({ tag: 'x' }), 'tag', 'x')]).toStrictEqual([
      { tag: 'x' },
      { tag: '' },
    ]);
  });
});

describe(newsPress, () => {
  it('leaves every new page: the view New, no pick, no search', () => {
    expect.assertions(1);
    expect(newsPress()).toStrictEqual({
      picks: { view: 'new', kind: '', template: '', project: '', tag: '' },
      q: '',
    });
  });
});

describe(newCount, () => {
  it('counts every unread page not archived, whatever narrows the list', () => {
    expect.assertions(1);
    expect(newCount([page('a', { news: 'new' }), page('b', { news: 'new', archived: true }), page('c')])).toBe(1);
  });
});

describe(narrowWords, () => {
  it('says how many picks on the button, and the view and the picks in the head', () => {
    expect.assertions(2);
    testBoot();
    expect(narrowWords(state({ tag: 'x', kind: 'link', view: 'fav' }))).toStrictEqual({
      n: 2,
      button: 'ui_narrow_n:2',
      what: 'ui_view_favorites:undefined · ui_narrow_n:2',
    });
    expect(narrowWords(state())).toStrictEqual({ n: 0, button: 'ui_narrow:undefined', what: '' });
  });
});

describe(listEmpty, () => {
  it('tells no page at all on All, archived ones neither, whatever the picks and the search', () => {
    expect.assertions(1);
    expect([
      listEmpty([], state(), none),
      listEmpty([page('a', { archived: true })], state(), none),
      listEmpty([page('a')], state({ q: 'nothing' }), none),
      listEmpty([], state({ view: 'fav' }), none),
    ]).toStrictEqual([true, false, false, false]);
  });
});

describe(tagPress, () => {
  it('narrows to the tag, and leaves the list as it is for the one picked already', () => {
    expect.assertions(1);
    expect([
      tagPress(state(), 'x'),
      tagPress(state({ tag: 'x' }), 'x'),
      tagPress(state({ tag: 'y' }), 'x'),
    ]).toStrictEqual([{ tag: 'x' }, null, { tag: 'x' }]);
  });
});

describe(unseenSlugs, () => {
  it('gives every unread page not archived, whatever the shelf picks', () => {
    expect.assertions(1);
    expect(
      unseenSlugs([
        page('a', { news: 'new' }),
        page('b'),
        page('c', { news: 'version', archived: true }),
        page('d', { news: 'version' }),
      ]),
    ).toStrictEqual(['a', 'd']);
  });
});
