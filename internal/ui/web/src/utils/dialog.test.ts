import { describe, expect, it } from 'vitest';

import { backdropDown } from '@/utils/dialog';

describe(backdropDown, () => {
  it('counts a press down on the backdrop of a modal not held, and none in its body or while it is held', () => {
    expect.assertions(3);
    expect(backdropDown(true, false)).toBe(true);
    expect(backdropDown(false, false)).toBe(false);
    expect(backdropDown(true, true)).toBe(false);
  });
});
