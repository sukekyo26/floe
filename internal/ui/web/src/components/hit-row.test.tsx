import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { HitRow } from '@/components/hit-row';
import { RowSub } from '@/components/row-sub';
import { testBoot } from '@/components/test-boot';
import type { Page } from '@/types/page';

const page = (change: Partial<Page>): Page => ({
  slug: 'a',
  title: 'A',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...change,
});
const ranks = new Map([['floe', 0]]);
const testids = (html: string) => [...html.matchAll(/data-testid="([^"]*)"/g)].map((m) => m[1]);

describe(RowSub, () => {
  it('tells a new version, the kind, the project of a colour, what the caller adds and the path, in that order', () => {
    expect.assertions(3);
    testBoot();
    const link = renderToString(
      <RowSub
        page={page({ news: 'version', version: 3, path: '/tmp/a.html', project: 'floe' })}
        ranks={ranks}
        middle={<b data-testid="badge" />}
      />,
    );
    expect(testids(link)).toStrictEqual(['ver', 'kindof', 'proj', 'proj-name', 'badge', 'path']);
    // The project's pill tells it is painted (P0's data-tint)
    expect(link).toMatch(/data-testid="proj"[^>]*data-tint="true"/);
    // A file gone is said in the path's place; a project with no colour, an archived page's news and a page of no
    // template say nothing
    const gone = renderToString(
      <RowSub
        page={page({ news: 'version', archived: true, path: '/tmp/a.html', missing: true, project: 'other' })}
        ranks={ranks}
      />,
    );
    expect(testids(gone)).toStrictEqual(['kindof', 'miss']);
  });
});

describe(HitRow, () => {
  it('is an option of the listbox, selected when reached, of the states the tests read', () => {
    expect.assertions(2);
    testBoot();
    const row = (reached: boolean, change: Partial<Page>) =>
      String(
        /<li[^>]*>/.exec(renderToString(<HitRow id="find-0" page={page(change)} reached={reached} sub={null} />))?.[0],
      );
    expect(row(true, { news: 'new' })).toMatch(/id="find-0" role="option" aria-selected="true" data-unread="true"/);
    expect(row(false, { archived: true })).toMatch(/aria-selected="false" data-archived="true"/);
  });
});
