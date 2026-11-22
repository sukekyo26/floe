import { describe, expect, it } from 'vitest';

import { droppedTo, dropsAfter, ends, focusStep, moved, nearDropped } from '@/features/tasks/utils/sortable';

describe(moved, () => {
  it('puts a row before another, as insertBefore does, from before it or after it, and last', () => {
    expect.assertions(4);
    expect(moved(['a', 'b', 'c'], 'c', 0)).toStrictEqual(['c', 'a', 'b']);
    expect(moved(['a', 'b', 'c'], 'a', 2)).toStrictEqual(['b', 'a', 'c']);
    expect(moved(['a', 'b', 'c'], 'a', 3)).toStrictEqual(['b', 'c', 'a']);
    expect(moved(['a', 'b', 'c'], 'b', 1)).toStrictEqual(['a', 'b', 'c']);
  });
});

describe(droppedTo, () => {
  it('is nothing where the drop leaves the rows as they were, as on the next row’s upper half', () => {
    expect.assertions(3);
    // b dropped before c (c's upper half) or after a (a's lower half): where it is
    expect(droppedTo(['a', 'b', 'c'], 'b', 2)).toBeNull();
    expect(droppedTo(['a', 'b', 'c'], 'b', 1)).toBeNull();
    expect(droppedTo(['a', 'b', 'c'], 'b', 0)).toStrictEqual(['b', 'a', 'c']);
  });
});

describe(focusStep, () => {
  it('keeps the button pressed, or gives the other once the row is at the end it moved to', () => {
    expect.assertions(4);
    expect(focusStep('up', 1, 3)).toBe('up');
    expect(focusStep('up', 0, 3)).toBe('down');
    expect(focusStep('down', 2, 3)).toBe('up');
    expect(focusStep('down', 1, 3)).toBe('down');
  });
});

describe(ends, () => {
  it('stops the first row going up and the last going down', () => {
    expect.assertions(1);
    expect([ends(0, 3), ends(1, 3), ends(2, 3)]).toStrictEqual([
      { up: true, down: false },
      { up: false, down: false },
      { up: false, down: true },
    ]);
  });
});

describe(nearDropped, () => {
  it('is the row after the one taken off, or before it, or none', () => {
    expect.assertions(3);
    expect(nearDropped(1, 3)).toBe(1);
    expect(nearDropped(2, 2)).toBe(1);
    expect(nearDropped(0, 0)).toBe(-1);
  });
});

describe(dropsAfter, () => {
  it('drops after a row in its lower half', () => {
    expect.assertions(2);
    expect(dropsAfter(100, 40, 121)).toBe(true);
    expect(dropsAfter(100, 40, 120)).toBe(false);
  });
});
