import { describe, expect, it } from 'vitest';

import { holdNext } from '@/hooks/use-hold';

describe(holdNext, () => {
  it('runs a write and holds, and does not run one asked for while held', () => {
    expect.assertions(2);
    expect(holdNext(false)).toStrictEqual({ holding: true, runs: true });
    expect(holdNext(true)).toStrictEqual({ holding: true, runs: false });
  });
});
