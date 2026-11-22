import { describe, expect, it } from 'vitest';

import { diffPickKey, keyName, plainKey, type KeyTarget } from '@/utils/keys';

const press = { key: 'j', ctrlKey: false, metaKey: false, altKey: false };
// A target that is in what the selector names when it is one of in
const at = (...in_: string[]): KeyTarget => ({
  closest: (selector) => (in_.some((x) => selector.includes(x)) ? {} : null),
  matches: () => false,
});

const checkBox: KeyTarget = {
  closest: (selector) => (selector.includes('input:not([type="checkbox"])') || !selector.includes('input') ? null : {}),
  matches: () => false,
};

// A choice of versions closed, the key on it; and one open, the key on its option, which is in it but no choice
const choice: KeyTarget = { closest: () => ({}), matches: (selector) => selector.includes('[data-version-pick]') };
const option: KeyTarget = { closest: () => ({}), matches: () => false };

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
  it('lets the moves and d go on from a choice of versions closed alone, not from its list open', () => {
    expect.assertions(4);
    expect(diffPickKey({ ...press, key: 'n', target: choice })).toBe(true);
    expect(diffPickKey({ ...press, key: 'v', target: choice })).toBe(false);
    expect(diffPickKey({ ...press, key: 'd', target: option })).toBe(false);
    expect(diffPickKey({ ...press, key: 'n', target: at() })).toBe(false);
  });
});
