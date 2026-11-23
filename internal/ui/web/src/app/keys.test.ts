import { describe, expect, it } from 'vitest';

import { appKeys, frameRows } from '@/app/keys';

const none = () => undefined;
const table = (up: (() => void) | undefined) =>
  Object.keys(
    appKeys({
      rows: { ArrowDown: none, ArrowUp: none, j: none, k: none },
      find: none,
      findToggle: none,
      help: none,
      star: none,
      diff: none,
      up,
    }),
  ).toSorted();

describe(appKeys, () => {
  it('has the rows, the search, the help, the star and the diff on both task screens, h on a task page alone, x on none', () => {
    expect.assertions(2);
    expect(table(undefined)).toStrictEqual(['/', '?', 'ArrowDown', 'ArrowUp', 'Ctrl+K', 'd', 'f', 'j', 'k']);
    expect(table(none)).toStrictEqual(['/', '?', 'ArrowDown', 'ArrowUp', 'Ctrl+K', 'd', 'f', 'h', 'j', 'k']);
  });

  it('sends / to the list’s search where the screen has one, else to the search modal, and x with the rows that have it', () => {
    expect.assertions(3);
    const called: string[] = [];
    const keys = (search?: () => void) =>
      appKeys({
        rows: { j: none, x: none },
        find: () => called.push('find'),
        search,
        findToggle: none,
        help: none,
        star: none,
        diff: none,
      });
    keys(() => called.push('search'))['/']?.();
    keys()['/']?.();
    expect(called).toStrictEqual(['search', 'find']);
    expect(keys()).toHaveProperty('x');
    expect(keys()).not.toHaveProperty('h');
  });
});

describe(frameRows, () => {
  it('moves j and k through the pages, and n and p through a diff’s changes in a diff alone, with no row keys', () => {
    expect.assertions(3);
    const moved: string[] = [];
    const keys = frameRows({
      move: (step) => moved.push(`page ${step}`),
      diff: (step) => moved.push(`change ${step}`),
    });
    for (const key of ['j', 'k', 'n', 'p']) keys[key]?.();
    expect(moved).toStrictEqual(['page 1', 'page -1', 'change 1', 'change -1']);
    expect(Object.keys(frameRows({ move: none, diff: null })).toSorted()).toStrictEqual(['j', 'k']);
    expect(
      Object.keys(
        appKeys({ rows: keys, find: none, findToggle: none, help: none, star: none, diff: none, up: none }),
      ).toSorted(),
    ).toStrictEqual(['/', '?', 'Ctrl+K', 'd', 'f', 'h', 'j', 'k', 'n', 'p']);
  });
});
