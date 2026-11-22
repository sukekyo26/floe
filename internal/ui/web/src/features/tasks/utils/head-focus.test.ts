import { describe, expect, it } from 'vitest';

import { firstShown, headFocus, stateShown } from '@/features/tasks/utils/head-focus';

describe(headFocus, () => {
  const none = { stateHidden: false, parentHidden: false, rebuilt: false };

  it('goes from the choices hidden to the archive button, from a parent button gone to the one shown', () => {
    expect.assertions(4);
    expect(headFocus({ on: 'state' }, { ...none, stateHidden: true })).toStrictEqual({ to: 'archive' });
    expect(headFocus({ on: 'state' }, none)).toBeNull();
    expect(headFocus({ on: 'parent' }, { ...none, parentHidden: true })).toStrictEqual({ to: 'part-up' });
    expect(headFocus({ on: 'parent' }, none)).toBeNull();
  });

  it('goes back to the same row and the same thing of it as the card is made anew, nowhere with nothing focused', () => {
    expect.assertions(3);
    expect(headFocus({ on: 'kin', task: '#4', unlink: true }, { ...none, rebuilt: true })).toStrictEqual({
      to: 'kin',
      task: '#4',
      unlink: true,
    });
    expect(headFocus({ on: 'kin', task: '#4', unlink: false }, none)).toBeNull();
    expect(headFocus(null, { stateHidden: true, parentHidden: true, rebuilt: true })).toBeNull();
  });
});

describe(stateShown, () => {
  const doing = { state: 'doing', rev: 1 };
  const shown = firstShown(doing, 5);

  it('holds what it shows while the choices are open or a change is sent', () => {
    expect.assertions(2);
    const done = { state: 'done', rev: 2 };
    expect(stateShown(shown, done, 6, true)).toBe(shown);
    const sending = { ...shown, chosen: 'todo' };
    expect(stateShown(sending, done, 6, false)).toBe(sending);
  });

  it('keeps a change written though the props of before the read after it come', () => {
    expect.assertions(2);
    const written = { ...shown, state: 'todo', rev: 2 };
    // The same props as before the write, the same reads: nothing to take
    expect(stateShown(written, doing, 5, false)).toBe(written);
    // The store's row is the answer's by then: taken, it is the same
    expect(stateShown(written, { state: 'todo', rev: 2 }, 5, false)).toMatchObject({ state: 'todo', rev: 2 });
  });

  it('takes the task once closed, at the next read, though nothing changed after it closed', () => {
    expect.assertions(3);
    const changed = { state: 'archived', rev: 2 };
    // Changed while open, held; the props come no more, a read of the same words ends after it closed
    const open = stateShown(shown, changed, 6, true);
    const closed = stateShown(open, changed, 7, false);
    expect([closed.state, closed.rev, closed.chosen]).toStrictEqual(['archived', 2, null]);
    expect(stateShown(closed, changed, 7, false)).toBe(closed);
    // A read with nothing new ends: the same as drawn
    expect(stateShown(shown, doing, 6, false)).toMatchObject({ state: 'doing', rev: 1, reads: 6 });
  });
});
