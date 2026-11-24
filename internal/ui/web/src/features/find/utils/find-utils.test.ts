import { describe, expect, it } from 'vitest';

import { findCount, findHits } from '@/features/find/utils/find-hits';
import { FIND_ROWS } from '@/utils/search';
import { FIRST, findKey, findReached } from '@/features/find/utils/find-state';
import type { Page } from '@/types/page';

const page = (slug: string, archived = false): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...(archived ? { archived } : {}),
});
const key = (k: string, isComposing = false) => ({ key: k, isComposing, keyCode: isComposing ? 229 : 0 });

describe(findHits, () => {
  it('puts the archived after the rest, each in the list’s order', () => {
    expect.assertions(1);
    const entries = [page('a1', true), page('b'), page('a2', true), page('c')];
    expect(findHits('', entries).map((a) => a.slug)).toStrictEqual(['b', 'c', 'a1', 'a2']);
  });
});

describe(findCount, () => {
  it('says the rows are cut, how many the words found, or the newest first', () => {
    expect.assertions(1);
    expect([findCount('', FIND_ROWS + 1), findCount('x', 3), findCount('', 0), findCount('', 3)]).toStrictEqual([
      { key: 'ui_find_cut', n: FIND_ROWS },
      { key: 'ui_find_n', n: 3 },
      { key: 'ui_find_n', n: 0 },
      { key: 'ui_find_newest', n: 3 },
    ]);
  });
});

describe(findReached, () => {
  it('keeps the same page, else what came to its place, else the last, and none of no rows', () => {
    expect.assertions(1);
    const shown = [page('a'), page('b'), page('c')];
    expect([
      findReached(shown, { slug: 'c', at: 0 }),
      findReached(shown, { slug: 'gone', at: 1 }),
      findReached(shown, { slug: 'gone', at: 9 }),
      findReached(shown, FIRST),
      findReached([], FIRST),
    ]).toStrictEqual([2, 1, 2, 0, -1]);
  });
});

describe(findKey, () => {
  it('moves by the arrows, opens by Enter on the field, and leaves a composed word’s keys alone', () => {
    expect.assertions(2);
    const free = { onField: true, reached: 0, menuOpen: false };
    expect([
      findKey(key('ArrowDown'), free),
      findKey(key('Enter'), free),
      findKey(key('Enter'), { ...free, onField: false }),
      findKey(key('Enter'), { ...free, reached: -1 }),
      findKey(key('Enter', true), free),
      findKey(key('ArrowDown', true), free),
      findKey(key('Escape'), free),
      findKey(key('Escape', true), free),
    ]).toStrictEqual(['move', 'open', 'none', 'none', 'none', 'none', 'close', 'none']);
    // A menu open holds the arrows, and Escape closes it alone
    const menu = { ...free, menuOpen: true };
    expect([findKey(key('ArrowUp'), menu), findKey(key('Escape'), menu), findKey(key('Enter'), menu)]).toStrictEqual([
      'hold',
      'closeMenu',
      'none',
    ]);
  });
});
