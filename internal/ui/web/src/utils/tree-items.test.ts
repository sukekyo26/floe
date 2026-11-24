import { describe, expect, it } from 'vitest';

import type { Listing } from '@/types/page';
import { treeItems } from '@/utils/tree-items';

const at = '2026-11-21T00:00:00Z';
const listing = (dirs: string[], files: string[], more = 0): Listing => ({
  dirs,
  files: files.map((file) => ({ file, updatedAt: at })),
  more,
});
const keys = (rows: ReturnType<typeof treeItems>) => rows.map((r) => `${r.key}@${r.d}`);

describe(treeItems, () => {
  it('draws the directories, each open one followed by its own rows one deeper, then the pages', () => {
    expect.assertions(2);
    const listings = new Map<string, Listing | Error>([
      ['f', listing(['a', 'b'], ['top.html'])],
      ['f:a', listing([], ['a/x.html'])],
      ['f:b', listing([], ['b/y.html'])],
    ]);
    const rows = treeItems('f', '', 1, listings, new Set(['f', 'f:a']));
    expect(keys(rows)).toStrictEqual(['dir:f:a@1', 'page:f:a/x.html@2', 'dir:f:b@1', 'page:f:top.html@1']);
    expect(rows.filter((r) => r.kind === 'dir').map((r) => r.open)).toStrictEqual([true, false]);
  });

  it('leaves out a directory not read yet, and draws nothing of a folder not read', () => {
    expect.assertions(2);
    const listings = new Map<string, Listing | Error>([['f', listing(['a'], [])]]);
    expect(keys(treeItems('f', '', 1, listings, new Set(['f', 'f:a'])))).toStrictEqual(['dir:f:a@1']);
    expect(treeItems('g', '', 1, listings, new Set(['g']))).toStrictEqual([]);
  });

  it('notes a directory unread, cut or empty after its rows, and nothing of one shown whole', () => {
    expect.assertions(1);
    const last = (l: Listing | Error) => treeItems('f', '', 1, new Map([['f', l]]), new Set(['f'])).at(-1)?.kind;
    expect([
      last(new Error('gone')),
      last(listing([], ['x.html'], 3)),
      last(listing([], [])),
      last(listing([], ['x.html'])),
    ]).toStrictEqual(['note', 'note', 'note', 'page']);
  });
});
