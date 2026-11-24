import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { pageTags, TagDialog } from '@/features/tags/components/tag-dialog';
import { TagLine } from '@/features/tags/components/tag-line';
import { createStore } from '@/lib/store';
import type { Pages } from '@/stores/pages';
import type { Page } from '@/types/page';

const page = (slug: string, tags: string[]): Page => ({
  slug,
  title: slug.toUpperCase(),
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  tags,
});
const none = () => undefined;
const chipOf = (html: string, name: string) =>
  String(
    new RegExp(
      `<label[^>]*data-testid="tag-box"[^>]*>(?:(?!</label>).)*>${name}</span>(?:(?!</label>).)*</label>`,
    ).exec(html)?.[0],
  );

describe(TagDialog, () => {
  it('draws a chip each tag, marked on all, some or none of the pages, with how many for some', () => {
    expect.assertions(5);
    testBoot();
    const pages = {
      state: createStore({ entries: [page('a', ['x', 'y']), page('b', ['x']), page('c', ['z'])] }),
    } as unknown as Pages;
    const target = { ...pageTags(page('a', []), () => false), slugs: ['a', 'b'] };
    const html = renderToString(<TagDialog target={target} onClosed={none} pages={pages} />);
    expect(chipOf(html, 'x')).toMatch(/checked=""[^>]*>.*data-testid="mark"[^>]*>✓/);
    expect(chipOf(html, 'y')).toMatch(/data-testid="mark"[^>]*>−.*data-testid="n"[^>]*>1\/2/);
    expect(chipOf(html, 'z')).toMatch(/data-testid="mark"[^>]*>\+/);
    // Checked is on all alone (some is indeterminate, set on the box itself), and how many is told for some alone
    expect([
      chipOf(html, 'y').includes('checked=""'),
      chipOf(html, 'z').includes('checked=""'),
      chipOf(html, 'x').includes('data-testid="n"'),
    ]).toStrictEqual([false, false, false]);
    // Apply is for a change, none made yet
    expect(html).toMatch(/<button[^>]*id="tag-apply"[^>]*disabled=""/);
  });
});

describe(TagLine, () => {
  it('pills each tag and says what its button does, and hides for no page', () => {
    expect.assertions(3);
    testBoot();
    const tagged = renderToString(<TagLine id="read-tags" page={page('a', ['x', 'y'])} hidden={false} onEdit={none} />);
    expect([...tagged.matchAll(/data-testid="tagpill"[^>]*>([^<]*)</g)].map((m) => m[1])).toStrictEqual(['x', 'y']);
    const bare = renderToString(<TagLine id="read-tags" page={page('a', [])} hidden={false} onEdit={none} />);
    expect([
      bare.includes('aria-label="ui_tag_add_named:A"'),
      bare.includes('data-tagged'),
      tagged.includes('data-tagged="true"'),
    ]).toStrictEqual([true, false, true]);
    expect(renderToString(<TagLine id="read-tags" page={null} hidden={false} onEdit={none} />)).toMatch(
      /^<div id="read-tags" hidden=""/,
    );
  });
});
