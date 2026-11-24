import { describe, expect, it } from 'vitest';

import { archiveEscapeTarget, rowAfter, rowNearby } from '@/utils/focus-targets';
import type { RowSeen } from '@/utils/row-focus';

const rows = (...keys: string[]): RowSeen[] => keys.map((key) => ({ key, titled: !key.startsWith('group') }));

describe(archiveEscapeTarget, () => {
  it('goes to its own row on a task’s page, else to the row kept as the one after, else to the last row', () => {
    expect.assertions(4);
    expect(archiveEscapeTarget(rows('a', 'b', 'c'), 'b', 'c', true)).toBe('b');
    // Gone from the list once it was read again: the row taken before
    expect(archiveEscapeTarget(rows('a', 'c'), 'b', 'c', false)).toBe('c');
    expect(archiveEscapeTarget(rows('a', 'group:x'), 'b', null, false)).toBe('a');
    expect(archiveEscapeTarget(rows('group:x'), 'b', 'c', false)).toBeNull();
  });
});

describe(rowAfter, () => {
  it('finds the next row with a title, past a heading, and none after the last', () => {
    expect.assertions(2);
    expect(rowAfter(rows('a', 'group:x', 'b'), 'a')).toBe('b');
    expect(rowAfter(rows('a', 'b'), 'b')).toBeNull();
  });
});

describe(rowNearby, () => {
  it('finds the row after, or with none after, the row before', () => {
    expect.assertions(3);
    expect(rowNearby(rows('a', 'b', 'c'), 'b')).toBe('c');
    expect(rowNearby(rows('a', 'group:x', 'b'), 'b')).toBe('a');
    expect(rowNearby(rows('b'), 'b')).toBeNull();
  });
});
