import { describe, expect, it } from 'vitest';

import { createMenus } from '@/stores/menus';

describe(createMenus, () => {
  it('keeps one menu open, and closes a menu only while it is the one open', () => {
    expect.assertions(3);
    const menus = createMenus();
    menus.show('a');
    menus.show('b');
    expect(menus.open.get()).toBe('b');
    // a's toggle comes late, after b took its place
    menus.hide('a');
    expect(menus.open.get()).toBe('b');
    menus.hide('b');
    expect(menus.open.get()).toBeNull();
  });
});
