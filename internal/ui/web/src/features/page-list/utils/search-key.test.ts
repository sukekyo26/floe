import { describe, expect, it } from 'vitest';

import { searchKey } from '@/features/page-list/utils/search-key';

describe(searchKey, () => {
  it('goes with the words on Enter where the field goes somewhere, and leaves the field on Escape', () => {
    expect.assertions(1);
    const done: string[] = [];
    const to = { go: (words: string) => done.push(`go ${words}`), leave: () => done.push('leave') };
    searchKey('Enter', 'a b', to);
    searchKey('Escape', 'a b', to);
    searchKey('a', 'a b', to);
    searchKey('Enter', 'a b', { leave: to.leave });
    expect(done).toStrictEqual(['go a b', 'leave']);
  });
});
