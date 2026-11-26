import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
// oxlint-disable-next-line import/no-relative-parent-imports -- the store's cases, which its Go test reads too
import taskURLs from '../../../../../../store/testdata/task-urls.json?raw';
import {
  boxSaid,
  categoryRefusal,
  editChange,
  mergeLists,
  mergeURLs,
  refillTitle,
  urlRefusal,
  urlLink,
  urlsFull,
} from '@/features/tasks/utils/task-edit';

const base = { name: 'S1', title: 'T', urls: ['u1', 'u2'], categories: ['a', 'b'] };

describe(editChange, () => {
  it('sends what changed alone, in the order the store reads, and nothing where nothing changed', () => {
    expect.assertions(4);
    expect(editChange(base, { name: 'S1', title: ' T ', urls: ['u1', 'u2'], categories: ['a', 'b'] })).toBeNull();
    // Added and taken off, the order kept: no order
    expect(
      JSON.stringify(editChange(base, { name: 'S1', title: 'N', urls: ['u2', 'u3'], categories: ['a', 'b'] })),
    ).toBe('{"title":"N","urls":{"add":["u3"],"remove":["u1"]}}');
    expect(editChange(base, { name: 'S1', title: 'T', urls: ['u2', 'u1'], categories: ['a', 'b'] })).toStrictEqual({
      urls: { order: ['u2', 'u1'] },
    });
    expect(editChange(base, { name: 'S1', title: 'T', urls: ['u1', 'u2'], categories: ['b', 'a', 'c'] })).toStrictEqual(
      {
        categories: ['b', 'a', 'c'],
      },
    );
  });

  it('sends the ID trimmed as rename when it changed alone, and an empty one for the store to refuse', () => {
    expect.assertions(3);
    const now = { title: 'T', urls: ['u1', 'u2'], categories: ['a', 'b'] };
    expect(editChange(base, { ...now, name: ' S1 ' })).toBeNull();
    expect(editChange(base, { ...now, name: ' S2 ' })).toStrictEqual({ rename: 'S2' });
    expect(editChange(base, { ...now, name: ' ' })).toStrictEqual({ rename: '' });
  });
});

describe(mergeLists, () => {
  it('takes the list read where it was not changed, else keeps the person’s with another’s added after', () => {
    expect.assertions(2);
    expect(mergeLists(['a', 'b'], ['a', 'b'], ['b', 'c'])).toStrictEqual(['b', 'c']);
    // Mine: b first, a removed by me, c added by me; another removed b and added d
    expect(mergeLists(['c', 'b'], ['a', 'b'], ['a', 'd'])).toStrictEqual(['c', 'd']);
  });
});

describe(mergeURLs, () => {
  it('keeps the order another saved where the person did not reorder, with the person’s added after', () => {
    expect.assertions(2);
    expect(mergeURLs(['u1', 'u2', 'u3'], ['u1', 'u2'], ['u2', 'u1'])).toStrictEqual(['u2', 'u1', 'u3']);
    // Reordered by the person: as mergeLists
    expect(mergeURLs(['u2', 'u1'], ['u1', 'u2'], ['u1', 'u2', 'u4'])).toStrictEqual(['u2', 'u1', 'u4']);
  });
});

describe(refillTitle, () => {
  it('takes the title read again where the field is as it opened, and keeps what was typed', () => {
    expect.assertions(2);
    expect(refillTitle(' T ', 'T', 'Theirs')).toBe('Theirs');
    expect(refillTitle('Mine', 'T', 'Theirs')).toBe('Mine');
  });
});

describe(urlRefusal, () => {
  it('refuses none, one not http nor https, one there and one past the most, and adds any other', () => {
    expect.assertions(4);
    testBoot();
    expect(urlRefusal('', [], 10)).toBe('ui_url_blank:undefined');
    expect(urlRefusal('https://a', ['https://a'], 10)).toBe('ui_url_there:https://a');
    expect(urlRefusal('https://b', ['x', 'y'], 2)).toBe('ui_url_max:2');
    expect(urlRefusal('ftp://a', ['x'], 10)).toBe('ui_url_not_link:ftp://a');
  });
});

describe(categoryRefusal, () => {
  it('refuses none and one there, and adds any other', () => {
    expect.assertions(3);
    testBoot();
    expect(categoryRefusal('', ['a'])).toBe('ui_category_blank:undefined');
    expect(categoryRefusal('a', ['a'])).toBe('ui_category_there:a');
    expect(categoryRefusal('b', ['a'])).toBe('');
  });
});

// The URLs the store's validURL is checked against too (internal/store/taskurl_internal_test.go): the modal's answer
// of each, as the browser reads it
const cases = JSON.parse(taskURLs) as { url: string; modal: boolean }[];

describe(urlLink, () => {
  it('takes what the browser reads as an http or https URL with a host, as the store’s cases say', () => {
    expect.assertions(1);
    expect(cases.filter((c) => urlLink(c.url.trim()) !== c.modal).map((c) => c.url)).toStrictEqual([]);
  });
});

describe(urlsFull, () => {
  it('is full at the most, which the refusal reads too', () => {
    expect.assertions(3);
    testBoot();
    expect(urlsFull(['a'], 2)).toBe(false);
    expect(urlsFull(['a', 'b'], 2)).toBe(true);
    expect(urlRefusal('https://c', ['a', 'b'], 2)).toBe('ui_url_max:2');
  });
});

describe(boxSaid, () => {
  it('says why it was not done, and nothing once anything is done to the list after', () => {
    expect.assertions(3);
    testBoot();
    expect(boxSaid({ act: 'full', max: 10 })).toBe('ui_url_max:10');
    expect(boxSaid({ act: 'paged', name: '設計' })).toBe('ui_category_drop_paged:設計');
    expect((['open', 'added', 'dropped', 'moved'] as const).map((act) => boxSaid({ act }))).toStrictEqual([
      '',
      '',
      '',
      '',
    ]);
  });
});
