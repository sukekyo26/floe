import { describe, expect, it } from 'vitest';

import { diffPickKey, heldKey, keyName, plainKey, type KeyTarget } from '@/utils/keys';

const press = { key: 'j', ctrlKey: false, metaKey: false, altKey: false };
// A target that is in what the selector names when it is one of in
const at = (...in_: string[]): KeyTarget => ({
  closest: (selector) => (in_.some((x) => selector.includes(x)) ? {} : null),
});

const checkBox: KeyTarget = {
  closest: (selector) => (selector.includes('input:not([type="checkbox"])') || !selector.includes('input') ? null : {}),
};

describe(plainKey, () => {
  it('takes a key on the page, not in a field, with a modal open or with a modifier, but in a check box', () => {
    expect.assertions(5);
    expect(plainKey({ ...press, target: at() }, false)).toBe(true);
    expect(plainKey({ ...press, target: at('textarea') }, false)).toBe(false);
    expect(plainKey({ ...press, target: at() }, true)).toBe(false);
    expect(plainKey({ ...press, ctrlKey: true, target: at() }, false)).toBe(false);
    // A check box is in a selector of inputs only where check boxes are not left out of it
    expect(plainKey({ ...press, target: checkBox }, false)).toBe(true);
  });
});

describe(keyName, () => {
  it('names Ctrl+K and ⌘K alike, and any other key by itself', () => {
    expect.assertions(3);
    expect(keyName({ ...press, key: 'k', ctrlKey: true })).toBe('Ctrl+K');
    expect(keyName({ ...press, key: 'k', metaKey: true })).toBe('Ctrl+K');
    expect(keyName({ ...press, key: 'k', ctrlKey: true, altKey: true })).toBe('k');
  });
});

describe(diffPickKey, () => {
  it('lets the moves and d go on from a choice of versions alone', () => {
    expect.assertions(3);
    expect(diffPickKey({ ...press, key: 'n' }, true)).toBe(true);
    expect(diffPickKey({ ...press, key: 'v' }, true)).toBe(false);
    expect(diffPickKey({ ...press, key: 'n' }, false)).toBe(false);
  });
});

describe(heldKey, () => {
  it('lets a held modal pass the keys with a modifier, the function keys and an input method’s, and no other', () => {
    expect.assertions(4);
    const key = { ...press, isComposing: false, keyCode: 0 };
    expect(heldKey({ ...key, key: 'F5' })).toBe(true);
    expect(heldKey({ ...key, key: 'r', ctrlKey: true })).toBe(true);
    expect(heldKey({ ...key, key: 'Enter', isComposing: true })).toBe(true);
    expect(heldKey({ ...key, key: 'Escape' })).toBe(false);
  });
});
