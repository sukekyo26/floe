import { describe, expect, it } from 'vitest';

import { armNext } from '@/hooks/use-arm';

describe(armNext, () => {
  it('arms on the first press, acts on the second and stays armed after it', () => {
    expect.assertions(2);
    expect(armNext(false)).toStrictEqual({ armed: true, act: false });
    expect(armNext(true)).toStrictEqual({ armed: true, act: true });
  });
});
