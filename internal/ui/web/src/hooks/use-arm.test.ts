import { describe, expect, it } from 'vitest';

import { armNext } from '@/hooks/use-arm';

describe(armNext, () => {
  it('arms on the first press, acts on the second and stays armed after it', () => {
    expect.assertions(2);
    expect(armNext(null, 'a')).toStrictEqual({ armedFor: 'a', act: false });
    expect(armNext('a', 'a')).toStrictEqual({ armedFor: 'a', act: true });
  });

  it('arms anew for another page rather than acting on it', () => {
    expect.assertions(1);
    expect(armNext('a', 'b')).toStrictEqual({ armedFor: 'b', act: false });
  });
});
