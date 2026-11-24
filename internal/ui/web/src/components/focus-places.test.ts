import { describe, expect, it } from 'vitest';

import { barMenuId, rowMenuId } from '@/components/focus-places';

describe(rowMenuId, () => {
  it('names a row’s menu by its page’s slug, so that a page renamed is found under its new one', () => {
    expect.assertions(1);
    expect([rowMenuId('a'), rowMenuId('b-2')]).toStrictEqual(['more:a', 'more:b-2']);
  });
});

describe(barMenuId, () => {
  it('names a bar’s menu by the bar, as BarActions draws it', () => {
    expect.assertions(1);
    expect(barMenuId('find-actions')).toBe('bar:find-actions');
  });
});
