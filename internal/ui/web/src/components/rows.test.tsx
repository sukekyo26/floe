import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { Rows, type RowItem } from '@/components/rows';
import { testBoot } from '@/components/test-boot';

const items: RowItem[] = [
  { kind: 'group', key: 'group:Today', heading: 'Today' },
  { kind: 'row', key: 'entry:a', titled: true, render: (kept) => <li key="a">{kept ? 'kept' : 'new'}</li> },
  { kind: 'group', key: 'group:Plans', heading: 'Plans', empty: 'none yet' },
];

describe(Rows, () => {
  it('draws #list with its headings as headings, a row new to it rising in, and tells while rows are selected', () => {
    expect.assertions(4);
    testBoot();
    const html = renderToString(<Rows items={items} selecting={false} />);
    expect(html).toMatch(/^<ul id="list"(?![^>]*data-selecting)/);
    expect(html).toMatch(
      /<li data-key="group:Today" data-testid="group" role="none"[^>]*><div role="heading" aria-level="2">Today<\/div>/,
    );
    expect(html).toMatch(/data-key="group:Plans".*data-testid="category-empty"[^>]*>none yet/s);
    expect([
      html.includes('<li>new</li>'),
      renderToString(<Rows items={items} selecting />).includes('data-selecting="true"'),
    ]).toStrictEqual([true, true]);
  });
});
