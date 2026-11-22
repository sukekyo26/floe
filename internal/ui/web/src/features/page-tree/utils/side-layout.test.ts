import { describe, expect, it } from 'vitest';

import { sideLayout } from '@/features/page-tree/utils/side-layout';

const wide = { open: true, over: false, narrow: false, width: 300, innerWidth: 1200 };

describe(sideLayout, () => {
  it('shows the tree beside the page as kept, in a narrow window over it as asked there, taking nothing', () => {
    expect.assertions(4);
    expect(sideLayout(wide)).toStrictEqual({ shown: true, width: 300, take: 300 });
    expect(sideLayout({ ...wide, open: false })).toStrictEqual({ shown: false, width: 300, take: 0 });
    expect(sideLayout({ ...wide, narrow: true, innerWidth: 500 })).toStrictEqual({ shown: false, width: 250, take: 0 });
    expect(sideLayout({ ...wide, open: false, narrow: true, over: true, innerWidth: 500 })).toStrictEqual({
      shown: true,
      width: 250,
      take: 0,
    });
  });

  it('holds what is kept within the bounds of a narrower window', () => {
    expect.assertions(1);
    expect(sideLayout({ ...wide, width: 480, innerWidth: 700 }).width).toBe(350);
  });
});
