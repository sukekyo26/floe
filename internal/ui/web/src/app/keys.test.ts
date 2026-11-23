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
});
