import { describe, expect, it } from 'vitest';

import { sideItems } from '@/features/page-tree/utils/side-items';
import type { Listing } from '@/types/page';

const listing = (dirs: string[], files: string[], more = 0): Listing => ({
  dirs,
  files: files.map((file) => ({ file, updatedAt: '' })),
  more,
});

describe(sideItems, () => {
  it('orders a directory, the rows under it open, its pages, then its note, keyed as the list’s tree', () => {
    expect.assertions(1);
    const listings = new Map<string, Listing | Error>([
      ['', listing(['api', 'guide'], ['index.html'])],
      ['guide', listing([], ['guide/a.html'], 3)],
      ['api', listing([], ['api/x.html'])],
    ]);
    const rows = sideItems('f', new Set(['', 'guide']), listings);
    expect(rows.map((r) => `${r.kind} ${r.key} ${r.d}`)).toStrictEqual([
      'dir dir:f:api 0',
      'dir dir:f:guide 0',
      'page page:f:guide/a.html 1',
      'note note:f:guide 1',
      'page page:f:index.html 0',
    ]);
  });

  it('says why a directory was not read, and draws none not read yet', () => {
    expect.assertions(2);
    expect(sideItems('f', new Set(['']), new Map([['', new Error('no')]])).map((r) => r.kind)).toStrictEqual(['note']);
    expect(sideItems('f', new Set(['']), new Map())).toStrictEqual([]);
  });
});
