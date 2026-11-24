import { describe, expect, it } from 'vitest';

import { bulkKind, seenOnly } from '@/features/page-list/utils/bulk';
import type { Query } from '@/stores/list-query';
import type { Page } from '@/types/page';

const q = (change: Partial<Query> = {}): Query => ({
  view: 'new',
  kind: '',
  template: '',
  project: '',
  tag: '',
  q: '',
  seen: new Set(['a']),
  ...change,
});

describe(seenOnly, () => {
  it('tells pages seen and nothing else, and no other change', () => {
    expect.assertions(1);
    expect([
      seenOnly(q(), q({ seen: new Set(['a', 'b']) })),
      seenOnly(q(), q({ seen: new Set(['a']) })),
      seenOnly(q(), q({ seen: new Set(['b', 'c']) })),
      seenOnly(q(), q({ seen: new Set(['a', 'b']), q: 'x' })),
      seenOnly(q(), q({ seen: new Set() })),
    ]).toStrictEqual([true, false, false, false, false]);
  });
});

describe(bulkKind, () => {
  it('unlinks when every one selected is a link of the list, and deletes otherwise', () => {
    expect.assertions(1);
    const items = [{ slug: 'l', path: '/x.html' }, { slug: 'p' }] as Page[];
    expect([bulkKind(['l'], items), bulkKind(['l', 'p'], items), bulkKind(['l', 'gone'], items)]).toStrictEqual([
      'unlink',
      'delete',
      'delete',
    ]);
  });
});
