import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import type { Openers } from '@/components/page-menu';
import { testBoot } from '@/components/test-boot';
import { FindDialog } from '@/features/find/components/find-dialog';
import { createStore } from '@/lib/store';
import type { Pages } from '@/stores/pages';
import type { Page } from '@/types/page';

const page = (slug: string, archived = false): Page => ({
  slug,
  title: slug.toUpperCase(),
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...(archived ? { archived } : {}),
});
const none = () => undefined;
const openers: Openers = { onTags: none, onTheme: none, onEdit: none, onTaskSet: none };

describe(FindDialog, () => {
  it('lists every page as options of the field’s listbox, the archived last with a badge, the first reached', () => {
    expect.assertions(4);
    testBoot();
    const pages = { state: createStore({ entries: [page('old', true), page('a'), page('b')] }) } as unknown as Pages;
    const html = renderToString(<FindDialog open={false} onClosed={none} openers={openers} pages={pages} />);
    expect(html).toMatch(
      /<input[^>]*id="find-q"[^>]*role="combobox"[^>]*aria-controls="find-list"[^>]*aria-activedescendant="find-0"/,
    );
    expect(html).toMatch(/<ul id="find-list" tabindex="-1" role="listbox"/);
    expect(
      [...html.matchAll(/<li id="(find-\d)"[^>]*aria-selected="(true|false)"/g)].map(
        (m) => `${String(m[1])}:${String(m[2])}`,
      ),
    ).toStrictEqual(['find-0:true', 'find-1:false', 'find-2:false']);
    expect(html).toMatch(/id="find-2"[^>]*data-archived="true".*data-testid="badge"/s);
  });
});
