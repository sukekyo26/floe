import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { TagPill } from '@/components/tag-pill';
import { testBoot } from '@/components/test-boot';
import { TreeRow } from '@/components/tree-row';
import type { TreeItem } from '@/utils/tree-items';

const folder = { slug: 'f', path: '/p' };
const draw = (item: TreeItem, more: { picked?: boolean; archived?: boolean } = {}) => {
  testBoot();
  return renderToString(<TreeRow item={item} folder={folder} {...more} />);
};
const li = (html: string) => String(/<li[^>]*>/.exec(html)?.[0]);

describe(TreeRow, () => {
  it('draws its three shapes, each with its time and none with a kind, a directory telling whether it is open', () => {
    expect.assertions(4);
    const dir = draw({ kind: 'dir', key: 'dir:f:d', d: 1, path: 'd', open: true });
    const page = draw(
      { kind: 'page', key: 'page:f:d/x.html', d: 2, file: 'd/x.html', updatedAt: '' },
      { picked: true },
    );
    const note = draw({ kind: 'note', key: 'note:f', d: 1, listing: { dirs: [], files: [], more: 0 } });
    expect([dir, page, note].map((html) => [html.includes('<time'), li(html).includes('data-kind')])).toStrictEqual([
      [true, false],
      [true, false],
      [true, false],
    ]);
    expect(dir).toMatch(/<button type="button" data-testid="title" aria-expanded="true" title="d"/);
    expect(li(page)).toMatch(
      /data-testid="tree" data-key="page:f:d\/x.html" data-d="2" data-slug="f" data-file="d\/x.html" aria-current="true"/,
    );
    expect(note).toContain('ui_folder_empty');
  });

  it('opens a page alone and copies its path, and dims the tree of a folder archived', () => {
    expect.assertions(3);
    const page = draw({ kind: 'page', key: 'page:f:x.html', d: 1, file: 'x.html', updatedAt: '' }, { archived: true });
    expect(page).toMatch(/data-testid="open"[^>]*href="\/a\/f\/x.html"/);
    expect(page).toContain('data-testid="copy"');
    expect(li(page)).toContain('data-archived="true"');
  });
});

describe(TagPill, () => {
  it('is a button that names what it narrows to with a press, and words alone without', () => {
    expect.assertions(2);
    testBoot();
    expect(renderToString(<TagPill name="x" onPress={() => undefined} />)).toMatch(
      /^<button type="button" data-testid="tagpill" title="x" aria-label="ui_tag_filter_named:x"/,
    );
    expect(renderToString(<TagPill name="x" />)).toMatch(/^<span data-testid="tagpill" title="x"/);
  });
});
