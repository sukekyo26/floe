import { describe, expect, it } from 'vitest';

import { backdropCloses, heldKey, stopsKey } from '@/utils/dialog';

describe(backdropCloses, () => {
  it('closes on a press down and up on the backdrop of a modal not held, and on no other', () => {
    expect.assertions(5);
    expect(backdropCloses({ onDialog: true, held: false }, true)).toBe(true);
    expect(backdropCloses({ onDialog: true, held: false }, false)).toBe(false);
    expect(backdropCloses({ onDialog: false, held: false }, true)).toBe(false);
    expect(backdropCloses({ onDialog: true, held: true }, true)).toBe(false);
    expect(backdropCloses(null, true)).toBe(false);
  });
});

describe(heldKey, () => {
  it('lets a held modal pass the keys with a modifier, the function keys and an input method’s, and no other', () => {
    expect.assertions(4);
    const key = { key: 'j', ctrlKey: false, metaKey: false, altKey: false, isComposing: false, keyCode: 0 };
    expect(heldKey({ ...key, key: 'F5' })).toBe(true);
    expect(heldKey({ ...key, key: 'r', ctrlKey: true })).toBe(true);
    expect(heldKey({ ...key, key: 'Enter', isComposing: true })).toBe(true);
    expect(heldKey({ ...key, key: 'Escape' })).toBe(false);
  });
});

describe(stopsKey, () => {
  it('stops a key once held, drawn or only begun in the same task, unless it passes', () => {
    expect.assertions(4);
    expect(stopsKey(false, false, false)).toBe(false);
    expect(stopsKey(true, false, false)).toBe(true);
    // Held in the task the key came in, before the modal is drawn held (TestUIDialogHold:248)
    expect(stopsKey(false, true, false)).toBe(true);
    expect(stopsKey(true, true, true)).toBe(false);
  });
});
