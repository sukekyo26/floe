import { describe, expect, it } from 'vitest';

import { readBoot } from '@/config/boot';

describe(readBoot, () => {
  it('reads the boot values, and refuses none and what is not JSON, saying to reload', () => {
    expect.assertions(4);
    expect(readBoot('{"screen":"list","framed":""}')).toStrictEqual({ screen: 'list', framed: '' });
    expect(() => readBoot(null)).toThrow('the page has no boot values: reload it');
    expect(() => readBoot('')).toThrow('the page has no boot values: reload it');
    expect(() => readBoot('{')).toThrow(/^the page has no boot values: reload it \(.+\)$/u);
  });
});
