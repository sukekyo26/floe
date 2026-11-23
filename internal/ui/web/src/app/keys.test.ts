import { describe, expect, it } from 'vitest';

import { appKeys } from '@/app/keys';

const none = () => undefined;
const table = (taskPage: boolean) =>
  Object.keys(
    appKeys({
      rows: { ArrowDown: none, ArrowUp: none, j: none, k: none },
      taskPage,
      find: none,
      findToggle: none,
      help: none,
      star: none,
      diff: none,
      up: none,
    }),
  ).toSorted();

describe(appKeys, () => {
  it('has the rows, the search, the help, the star and the diff on both task screens, h on a task page alone, x on none', () => {
    expect.assertions(2);
    expect(table(false)).toStrictEqual(['/', '?', 'ArrowDown', 'ArrowUp', 'Ctrl+K', 'd', 'f', 'j', 'k']);
    expect(table(true)).toStrictEqual(['/', '?', 'ArrowDown', 'ArrowUp', 'Ctrl+K', 'd', 'f', 'h', 'j', 'k']);
  });

  it('sends / to the list’s search where the screen has one, else to the search modal, and x with the rows that have it', () => {
    expect.assertions(3);
    const called: string[] = [];
    const keys = (search?: () => void) =>
      appKeys({
        rows: { j: none, x: none },
        taskPage: false,
        find: () => called.push('find'),
        search,
        findToggle: none,
        help: none,
        star: none,
        diff: none,
        up: none,
      });
    keys(() => called.push('search'))['/']?.();
    keys()['/']?.();
    expect(called).toStrictEqual(['search', 'find']);
    expect(keys()).toHaveProperty('x');
    expect(keys()).not.toHaveProperty('h');
  });
});
