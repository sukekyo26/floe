import { describe, expect, it } from 'vitest';

import { nextReached } from '@/utils/list-keys';

describe(nextReached, () => {
  it('goes down and up round the ends from none reached, and stays with no rows or another key', () => {
    expect.assertions(1);
    expect([
      nextReached(-1, 3, 'ArrowDown'),
      nextReached(2, 3, 'ArrowDown'),
      nextReached(-1, 3, 'ArrowUp'),
      nextReached(0, 3, 'ArrowUp'),
      nextReached(2, 3, 'ArrowUp'),
      nextReached(-1, 0, 'ArrowDown'),
      nextReached(1, 3, 'Enter'),
    ]).toStrictEqual([0, 0, 2, 2, 1, -1, 1]);
  });
});
