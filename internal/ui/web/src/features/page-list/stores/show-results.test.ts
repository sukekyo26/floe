import { describe, expect, it } from 'vitest';

import { showResults } from '@/features/page-list/stores/show-results';
import type { ColumnsStore } from '@/stores/columns';

const columns = (listOpen: boolean) => {
  const log: string[] = [];
  const store = {
    listOpen: () => listOpen,
    set: (change: object) => log.push(`set ${JSON.stringify(change)}`),
    keep: () => log.push('keep'),
  } as unknown as ColumnsStore;
  return { store, log };
};

describe(showResults, () => {
  it('opens the list folded and keeps it so, on the wide page alone', () => {
    expect.assertions(1);
    const folded = columns(false);
    showResults(folded.store, () => true);
    const open = columns(true);
    showResults(open.store, () => true);
    const narrow = columns(false);
    showResults(narrow.store, () => false);
    expect([folded.log, open.log, narrow.log]).toStrictEqual([['set {"listOpen":true}', 'keep'], [], []]);
  });
});
