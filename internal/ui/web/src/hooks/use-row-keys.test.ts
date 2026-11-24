import { describe, expect, it, vi } from 'vitest';

import { rowKeys } from '@/hooks/use-row-keys';

describe(rowKeys, () => {
  it('ticks the row’s box by x where it is given, and has no x where it is not', () => {
    expect.assertions(2);
    const tick = vi.fn<() => void>();
    const keys = rowKeys({ move: () => null, tick });
    keys['x']?.();
    expect(tick).toHaveBeenCalledTimes(1);
    expect(Object.keys(rowKeys({ move: () => null }))).not.toContain('x');
  });
});
