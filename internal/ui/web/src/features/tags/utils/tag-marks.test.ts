import { describe, expect, it } from 'vitest';

import {
  changed,
  changes,
  chips,
  fresh,
  nextMark,
  startMarks,
  tagNames,
  taken,
  toSend,
  type Mark,
} from '@/features/tags/utils/tag-marks';
import type { Page } from '@/types/page';

const page = (slug: string, tags?: string[]): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...(tags ? { tags } : {}),
});
const marks = (pairs: Record<string, Mark>) => new Map(Object.entries(pairs));

describe(startMarks, () => {
  it('marks a tag on all of the pages all, on some some and on none none', () => {
    expect.assertions(2);
    const pages = [page('a', ['x', 'y']), page('b', ['x'])];
    const got = startMarks(pages, tagNames([...pages, page('c', ['z'])], 'en'));
    expect([...got.start]).toStrictEqual([
      ['x', 'all'],
      ['y', 'some'],
      ['z', 'none'],
    ]);
    expect([...got.on]).toStrictEqual([
      ['x', 2],
      ['y', 1],
      ['z', 0],
    ]);
  });
});

describe(nextMark, () => {
  it('goes all ⇄ none, and back to some only for a chip that opened on some', () => {
    expect.assertions(1);
    expect([
      nextMark('all', 'all'),
      nextMark('none', 'all'),
      nextMark('some', 'some'),
      nextMark('all', 'some'),
      nextMark('none', 'some'),
    ]).toStrictEqual(['none', 'all', 'all', 'none', 'some']);
  });
});

describe(changes, () => {
  it('adds those changed to all, removes those changed to none, and leaves the rest; Apply is for a change', () => {
    expect.assertions(3);
    const start = marks({ a: 'none', b: 'some', c: 'all', d: 'some' });
    const want = marks({ a: 'all', b: 'none', c: 'all', d: 'some' });
    expect(changes(start, want)).toStrictEqual({ add: ['a'], remove: ['b'] });
    expect(changed(start, want)).toBe(true);
    expect(changed(start, new Map(start))).toBe(false);
  });
});

describe(toSend, () => {
  it('sends the pages still listed that the change would change', () => {
    expect.assertions(1);
    const now = [page('a', ['x']), page('b'), page('c', ['y'])];
    expect(toSend(now, ['a', 'b', 'c', 'gone'], ['x'], ['y'])).toStrictEqual(['b', 'c']);
  });
});

describe(chips, () => {
  it('draws the tags on its pages first, then by name, those holding the find words whatever the case', () => {
    expect.assertions(2);
    const want = marks({ beta: 'none', Alpha: 'none', gamma: 'all' });
    const on = new Map([
      ['beta', 0],
      ['Alpha', 0],
      ['gamma', 2],
    ]);
    expect(chips(want, on, '', 'en')).toStrictEqual(['gamma', 'Alpha', 'beta']);
    expect(chips(want, on, ' AL ', 'en')).toStrictEqual(['Alpha']);
  });
});

describe(fresh, () => {
  it('makes words a tag only when no tag has the name, whatever the case', () => {
    expect.assertions(1);
    const want = marks({ Foo: 'none' });
    expect([fresh(want, 'foo'), fresh(want, 'Foo'), fresh(want, 'food'), fresh(want, '  ')]).toStrictEqual([
      false,
      false,
      true,
      false,
    ]);
  });
});

describe(taken, () => {
  it('takes the tag of the same name whatever the case, else the one chip left, else none', () => {
    expect.assertions(1);
    expect([
      taken(['Foo', 'Foobar'], 'foo'),
      taken(['Foo', 'Foobar'], 'Foo'),
      taken(['Foobar'], 'foob'),
      taken(['Foo', 'Foobar'], 'fo'),
      taken([], 'new'),
      taken(['Foo'], ' '),
    ]).toStrictEqual(['Foo', 'Foo', 'Foobar', null, null, null]);
  });

  it('takes, of two tags differing in case, the one written as typed, else the first drawn', () => {
    expect.assertions(1);
    expect([taken(['API', 'api'], 'api'), taken(['API', 'api'], 'Api')]).toStrictEqual(['api', 'API']);
  });
});
