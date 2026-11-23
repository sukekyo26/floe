import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import type { Boot } from '@/config/boot';
import { SearchBox } from '@/features/page-list/components/search-box';
import { createListQuery } from '@/stores/list-query';

// The list's query of a tab that searched x
const query = () =>
  createListQuery({
    boot: () => ({ screen: 'list', states: [] }) as unknown as Boot,
    search: () => '?q=x',
    storage: { read: () => null, write: () => undefined, remove: () => undefined },
  });

describe(SearchBox, () => {
  it("shows the list's words, and none of them where it goes somewhere with its own", () => {
    expect.assertions(2);
    testBoot();
    expect(renderToString(<SearchBox query={query()} />)).toMatch(/id="q"[^>]* value="x"/);
    expect(renderToString(<SearchBox query={query()} go={() => undefined} />)).toMatch(/id="q"[^>]* value=""/);
  });
});
