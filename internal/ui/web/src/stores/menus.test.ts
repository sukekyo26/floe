import { describe, expect, it } from 'vitest';

import { closedThen, createMenus } from '@/stores/menus';

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

describe(closedThen, () => {
  it('closes the menu of a page gone before what comes after', () => {
    expect.assertions(1);
    const menus = createMenus();
    menus.show('bar:b');
    const log: (string | null)[] = [];
    closedThen(menus, 'bar:b', () => log.push(menus.open.get()))();
    expect(log).toStrictEqual([null]);
  });
});
