import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { createOpenPage } from '@/stores/open-page';

const boot =
  (screen: Boot['screen'], framed: string, version = ''): (() => Boot) =>
  () =>
    ({ screen, framed, version }) as Boot;

describe(createOpenPage, () => {
  it('shows the page picked beside a list, whose framed is empty, and the frame its own', () => {
    expect.assertions(3);
    const list = createOpenPage({ boot: boot('list', ''), search: () => '' });
    expect(list.shownSlug()).toBeUndefined();
    list.picked.set({ slug: 'a', file: '', v: 0 });
    expect(list.shownSlug()).toBe('a');
    const frame = createOpenPage({ boot: boot('frame', 'f', '3'), search: () => '?file=x.html' });
    expect({ shown: frame.shownSlug(), at: frame.frameAt() }).toStrictEqual({
      shown: 'f',
      at: { slug: 'f', file: 'x.html', v: 3 },
    });
  });

  it('reads no version of a diff, and tells a page picked by its slug and file', () => {
    expect.assertions(4);
    const diff = createOpenPage({ boot: boot('frame', 'f', '3'), search: () => '?diff' });
    expect(diff.frameAt()?.v).toBe(0);
    // A folder's file is no diff, ?diff or not; a list is none
    const others = [
      createOpenPage({ boot: boot('frame', 'f'), search: () => '?file=x.html&diff' }),
      createOpenPage({ boot: boot('list', ''), search: () => '?diff' }),
    ];
    expect([diff.diff(), ...others.map((o) => o.diff())]).toStrictEqual([true, false, false]);
    diff.picked.set({ slug: 'a', file: 'b.html', v: 0 });
    expect(diff.isPicked({ slug: 'a', file: 'b.html' })).toBe(true);
    expect(diff.isPicked({ slug: 'a' })).toBe(false);
  });
});
