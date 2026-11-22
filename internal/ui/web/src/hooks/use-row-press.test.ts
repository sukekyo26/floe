import type { KeyboardEvent, MouseEvent } from 'react';
import { describe, expect, it, vi } from 'vitest';

import type { Boot } from '@/config/boot';
import { rowPresses } from '@/hooks/use-row-press';
import { rowKeys } from '@/hooks/use-row-keys';
import { createStore } from '@/lib/store';
import { createMenus } from '@/stores/menus';
import { createOpenPage } from '@/stores/open-page';
import type { Order } from '@/stores/order';
import type { Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { MovedTo } from '@/utils/row-press';

const page = (slug: string, change: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...change,
});
const made = (narrow = false) => {
  const openPage = createOpenPage({ boot: () => ({ screen: 'list', framed: '' }) as Boot, search: () => '' });
  const kept: [string[], string[]][] = [];
  const went: string[] = [];
  const menus = createMenus();
  const presses = rowPresses({
    selecting: () => false,
    drawn: () => [page('a'), page('f', { folder: true }), page('b')],
    openPage,
    order: { keepRows: (o: string[], s: string[]) => kept.push([o, s]) } as unknown as Order,
    menus,
    narrow: () => narrow,
    assign: (url) => went.push(url),
  });
  return { openPage, kept, went, menus, presses };
};
const click = (href: string | null, change: Partial<globalThis.MouseEvent> = {}) => {
  const preventDefault = vi.fn<() => void>();
  const e = {
    button: 0,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    ...change,
    preventDefault,
    currentTarget: { hasAttribute: (name: string) => name === 'href' && href !== null, href: href ?? '' },
  } as unknown as MouseEvent<HTMLAnchorElement>;
  return { e, preventDefault };
};

describe(rowPresses, () => {
  it('opens a row beside the list, and whole at its version pressed again, keeping the rows drawn', () => {
    expect.assertions(4);
    const { openPage, kept, went, presses } = made();
    const first = click('http://x/a/a/');
    presses.press({ slug: 'a', file: '' }, first.e);
    expect([openPage.picked.get(), first.preventDefault.mock.calls.length]).toStrictEqual([
      { slug: 'a', file: '', v: 0 },
      1,
    ]);
    openPage.picked.set({ slug: 'a', file: '', v: 3 });
    presses.press({ slug: 'a', file: '' }, click('http://x/a/a/').e);
    expect(went).toStrictEqual(['/a/a/?v=3']);
    expect(kept).toStrictEqual([
      [
        ['a', 'b'],
        ['a', 'f', 'b'],
      ],
    ]);
    // Modified, the link's own: nothing kept, nothing changed
    const modified = click('http://x/a/b/', { ctrlKey: true });
    presses.press({ slug: 'b', file: '' }, modified.e);
    expect([modified.preventDefault.mock.calls.length, kept.length]).toStrictEqual([0, 1]);
  });

  it('keeps the rows and leaves the link to go whole in one column, and opens beside with a menu open', () => {
    expect.assertions(2);
    const narrow = made(true);
    const one = click('http://x/a/a/');
    narrow.presses.press({ slug: 'a', file: '' }, one.e);
    expect([one.preventDefault.mock.calls.length, narrow.kept.length, narrow.openPage.picked.get()]).toStrictEqual([
      0,
      1,
      null,
    ]);
    const wide = made();
    wide.openPage.picked.set({ slug: 'a', file: '', v: 0 });
    wide.menus.show('more:x');
    wide.presses.press({ slug: 'a', file: '' }, click('http://x/a/a/').e);
    expect(wide.went).toStrictEqual([]);
  });

  it('opens whole on Enter, by the link of a row not open beside the list', () => {
    expect.assertions(2);
    const { went, kept, presses } = made();
    const enter = {
      key: 'Enter',
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      preventDefault: () => undefined,
      nativeEvent: { isComposing: false, keyCode: 13 },
      currentTarget: { hasAttribute: (name: string) => name === 'href', href: 'http://x/a/b/' },
    } as unknown as KeyboardEvent<HTMLAnchorElement>;
    presses.key({ slug: 'b', file: '' }, { ...enter, key: 'a' });
    presses.key({ slug: 'b', file: '' }, enter);
    expect(went).toStrictEqual(['http://x/a/b/']);
    expect(kept).toHaveLength(1);
  });
});

describe(rowKeys, () => {
  it('opens beside the row a key moves to where it has a page, and nothing of a directory or a task list’s row', () => {
    expect.assertions(1);
    const openPage = createOpenPage({ boot: () => ({ screen: 'list', framed: '' }) as Boot, search: () => '' });
    const to = [
      { slug: 'f', file: '', dir: true },
      { slug: '', file: '', dir: false },
      { slug: 'f', file: 'x.html', dir: false },
    ];
    const keys = rowKeys({
      pages: { state: createStore({ entries: [page('f', { folder: true })] }) } as unknown as Pages,
      openPage,
      move: () => to.shift() as MovedTo,
      narrow: () => false,
    });
    const seen: unknown[] = [];
    for (const name of ['j', 'ArrowDown', 'k']) {
      keys[name]?.();
      seen.push(openPage.picked.get());
    }
    expect(seen).toStrictEqual([null, null, { slug: 'f', file: 'x.html', v: 0 }]);
  });
});
