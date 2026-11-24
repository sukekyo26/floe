import { describe, expect, it } from 'vitest';

import { menusToClose } from '@/utils/menu';

describe(menusToClose, () => {
  it('closes the menu open on a click elsewhere or on an item done, and keeps it on its own button or a first delete', () => {
    expect.assertions(5);
    expect(menusToClose('a', { menu: null, item: false, keepsOpen: false })).toBe('a');
    expect(menusToClose('a', { menu: 'b', item: false, keepsOpen: false })).toBe('a');
    expect(menusToClose('a', { menu: 'a', item: true, keepsOpen: false })).toBe('a');
    expect(menusToClose('a', { menu: 'a', item: false, keepsOpen: false })).toBeNull();
    expect(menusToClose('a', { menu: 'a', item: true, keepsOpen: true })).toBeNull();
  });

  it('closes nothing with no menu open', () => {
    expect.assertions(1);
    expect(menusToClose(null, { menu: null, item: false, keepsOpen: false })).toBeNull();
  });
});
