import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { createStore } from '@/lib/store';
import type { Kept } from '@/lib/storage';
import { createFrameMoves } from '@/features/page-tree/stores/frame-moves';
import { createListQuery } from '@/stores/list-query';
import type { Folders } from '@/stores/open-folders';
import type { OpenAt } from '@/stores/open-page';
import { createOrder } from '@/stores/order';
import type { Pages } from '@/stores/pages';
import type { Page } from '@/types/page';

const memory = (start: Record<string, string> = {}): Kept => {
  const held = new Map(Object.entries(start));
  return {
    read: (key) => held.get(key) ?? null,
    write: (key, value) => held.set(key, value),
    remove: (key) => held.delete(key),
  };
};

const entry = (slug: string, more: Partial<Page> = {}): Page => ({
  slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...more,
});

// The entries in the server's order: pages of two projects, a link, an archived page, one gone and a folder
const ENTRIES = [
  entry('a', { project: 'p' }),
  entry('b', { project: 'q' }),
  entry('c', { project: 'p' }),
  entry('l1', { path: '/x/one.html' }),
  entry('l2', { path: '/x/two.html' }),
  entry('old', { archived: true }),
  entry('gone', { missing: true }),
  entry('dir', { folder: true }),
];

function moves(at: OpenAt, kept: Record<string, string> = {}, files: string[] = []) {
  const storage = memory(kept);
  const boot = () => ({ screen: 'frame', framed: at.slug, states: [] }) as unknown as Boot;
  const went: string[] = [];
  const order = createOrder({ boot, storage, here: () => `/a/${at.slug}/` });
  const fileListing = { dirs: [], files: files.map((file) => ({ file, updatedAt: '' })), more: 0 };
  const frame = createFrameMoves({
    boot,
    order,
    listQuery: createListQuery({ boot, search: () => '', storage }),
    pages: { state: createStore({ entries: ENTRIES }) } as unknown as Pages,
    folders: { side: createStore({ opened: new Set(), listings: new Map(), fileListing }) } as unknown as Folders,
    openPage: { frameAt: () => at },
    go: (href) => went.push(href),
  });
  return { frame, went, order };
}

const page = (slug: string): OpenAt => ({ slug, file: '', v: 0 });
const keptRows = (order: string[]) => ({
  'pages.order': JSON.stringify({ order, shown: order, back: '/?view=fav' }),
});

describe(createFrameMoves, () => {
  it('goes through the rows that opened the page, the ones gone since left out', () => {
    expect.assertions(2);
    const { frame, went } = moves(page('a'), keptRows(['c', 'gone', 'a', 'nowhere', 'b']));
    frame.moveFrame(1);
    frame.moveFrame(-1);
    expect(went).toStrictEqual(['/a/b/', '/a/c/']);
    const end = moves(page('b'), keptRows(['a', 'b']));
    end.frame.moveFrame(1);
    expect(end.went).toStrictEqual([]);
  });

  it('opened otherwise, goes through the list it was last on, or all entries of its kind', () => {
    expect.assertions(3);
    const saved = moves(page('a'), { 'pages.list': '?project=p' });
    saved.frame.moveFrame(1);
    expect(saved.went).toStrictEqual(['/a/c/']);
    const link = moves(page('l1'), { 'pages.list': '?project=p' });
    link.frame.moveFrame(1);
    expect(link.went).toStrictEqual(['/a/l2/']);
    const archived = moves(page('old'));
    archived.frame.moveFrame(1);
    expect(archived.went).toStrictEqual([]);
  });

  it('keeps the order first found, though what was kept goes', () => {
    expect.assertions(1);
    const { frame, went, order } = moves(page('a'), keptRows(['a', 'b']));
    frame.moveFrame(1);
    order.forgetOrder();
    frame.moveFrame(1);
    expect(went).toStrictEqual(['/a/b/', '/a/b/']);
  });

  it("goes through a folder's pages of its directory by name, not the list", () => {
    expect.assertions(1);
    const { frame, went } = moves({ slug: 'dir', file: 'docs/b.html', v: 0 }, keptRows(['a', 'dir', 'b']), [
      'docs/a.html',
      'docs/b.html',
      'docs/c.html',
    ]);
    frame.moveFrame(1);
    frame.moveFrame(-1);
    expect(went).toStrictEqual(['/a/dir/?file=docs%2Fc.html', '/a/dir/?file=docs%2Fa.html']);
  });
});
