import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { BarActions, RowActions } from '@/components/page-actions';
import type { Openers } from '@/components/page-menu';
import { testBoot } from '@/components/test-boot';
import type { Page } from '@/types/page';
import type { PageAt } from '@/utils/page-urls';

const none = () => undefined;
const openers: Openers = { onTags: none, onTheme: none, onEdit: none, onTaskSet: none };
const page: Page = {
  slug: 'a',
  title: 'A',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'floe',
  version: 3,
  versionCount: 3,
};
const tagOf = (html: string, testid: string) =>
  String(new RegExp(`<[a-z]+[^>]*data-testid="${testid}"[^>]*>`).exec(html)?.[0]);
const bar = (place: 'list' | 'frame', at: PageAt, a: Partial<Page> = {}, diffing = false) => {
  testBoot();
  return renderToString(
    <BarActions id="b" place={place} page={{ ...page, ...a }} at={at} openers={openers} diffing={diffing} />,
  );
};

describe(RowActions, () => {
  it('tells archived on and a star pressed by their attributes', () => {
    expect.assertions(3);
    testBoot();
    const html = renderToString(<RowActions page={{ ...page, archived: true, favorite: true }} openers={openers} />);
    expect(tagOf(html, 'archive')).toMatch(/data-on="true"/);
    expect(tagOf(html, 'archive')).not.toMatch(/ aria-pressed=/);
    expect(tagOf(html, 'star')).toMatch(/aria-pressed="true"/);
  });
});

describe(BarActions, () => {
  it('folds a page’s bar into its menu in a narrow frame, and leaves a folder’s page, which has no menu, as it is', () => {
    expect.assertions(4);
    const framed = bar('frame', { slug: 'a' });
    expect(
      ['download', 'open', 'archive', 'star'].map((id) => tagOf(framed, id).includes('narrow:hidden')),
    ).toStrictEqual([true, true, true, true]);
    const folder = bar('frame', { slug: 'a', file: 'docs/index.html' }, { folder: true, path: '/tmp/a' });
    expect(folder).not.toContain('data-testid="more"');
    expect(tagOf(folder, 'open')).not.toContain('narrow:hidden');
    expect(tagOf(bar('list', { slug: 'a' }), 'open')).not.toContain('narrow:hidden');
  });

  it('opens the diff of a page of versions, and closes it in the diff', () => {
    expect.assertions(4);
    expect(tagOf(bar('list', { slug: 'a', v: 2 }), 'diff-open')).toMatch(
      /href="\/a\/a\/\?diff&amp;from=2&amp;to=3"(?![^>]*hidden)/,
    );
    expect(tagOf(bar('list', { slug: 'a' }, { versionCount: 1 }), 'diff-open')).toContain('hidden=""');
    expect(tagOf(bar('frame', { slug: 'a' }, {}, true), 'diff-open')).toMatch(
      /href="\/a\/a\/".*data-testid="diff-open"|data-testid="diff-open".*href="\/a\/a\/"/,
    );
    expect(tagOf(bar('frame', { slug: 'a' }, {}, true), 'diff-open')).toContain('data-on="true"');
  });
});
