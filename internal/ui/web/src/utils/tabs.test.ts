import { describe, expect, it } from 'vitest';

import { nextTab } from '@/utils/tabs';

describe(nextTab, () => {
  it('goes right and left round the ends, and stays for any other key', () => {
    expect.assertions(1);
    expect([
      nextTab(0, 2, 'ArrowRight'),
      nextTab(1, 2, 'ArrowRight'),
      nextTab(0, 2, 'ArrowLeft'),
      nextTab(1, 3, 'ArrowLeft'),
      nextTab(1, 2, 'Enter'),
      nextTab(1, 3, 'ArrowRight'),
      nextTab(2, 3, 'ArrowRight'),
      nextTab(0, 3, 'ArrowLeft'),
    ]).toStrictEqual([1, 0, 1, 0, 1, 2, 0, 2]);
  });
});
