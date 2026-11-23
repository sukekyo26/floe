import { describe, expect, it } from 'vitest';

import { freeOf } from '@/app/modals';

describe(freeOf, () => {
  it('is free with no modal open, or with only the one closing in the same press', () => {
    expect.assertions(5);
    expect(freeOf([])).toBe(true);
    expect(freeOf(['keys'])).toBe(false);
    expect(freeOf(['find'], 'find')).toBe(true);
    expect(freeOf(['find', 'keys'], 'find')).toBe(false);
    expect(freeOf(['keys'], 'find')).toBe(false);
  });
});
