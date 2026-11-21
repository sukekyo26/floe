import { describe, expect, it } from 'vitest';

import { focusTarget, type RowSeen } from '@/utils/row-focus';

const rows = (...keys: string[]): RowSeen[] => keys.map((key) => ({ key, titled: !key.startsWith('group') }));

describe(focusTarget, () => {
  it('keeps the focus on what had it in a row that stayed', () => {
    expect.assertions(3);
    const was = ['a', 'b', 'c'];
    expect(focusTarget(was, rows('a', 'b', 'c'), { key: 'b', on: 'star' })).toStrictEqual({ key: 'b', on: 'star' });
    expect(focusTarget(was, rows('b', 'a', 'c'), { key: 'b', on: 'summary' })).toStrictEqual({
      key: 'b',
      on: 'summary',
    });
    // A new row before it does not move it among the rows there before and after
    expect(focusTarget(was, rows('n', 'a', 'b', 'c'), { key: 'b', on: 'archive' })).toStrictEqual({
      key: 'b',
      on: 'archive',
    });
  });

  it('moves a button’s focus from a row that moved on, or a row gone, to the title of the row after it, or the last', () => {
    expect.assertions(4);
    const was = ['a', 'b', 'c'];
    expect(focusTarget(was, rows('a', 'c', 'b'), { key: 'b', on: 'archive' })).toStrictEqual({ key: 'c', on: 'title' });
    expect(focusTarget(was, rows('a', 'c'), { key: 'b', on: 'title' })).toStrictEqual({ key: 'c', on: 'title' });
    expect(focusTarget(was, rows('a', 'b', 'group:x'), { key: 'c', on: 'title' })).toStrictEqual({
      key: 'b',
      on: 'title',
    });
    expect(focusTarget(was, rows('group:x'), { key: 'c', on: 'title' })).toBeNull();
  });
});
