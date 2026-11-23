import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { PageRow } from '@/components/page-row';
import { testBoot } from '@/components/test-boot';
import type { Page } from '@/types/page';

const page: Page = {
  slug: 'a',
  title: 'A page',
  description: 'what it is',
  project: 'floe',
  createdAt: '',
  updatedAt: '2026-11-21T00:00:00Z',
  taskProject: 'floe',
};
const draw = (p: Partial<Parameters<typeof PageRow>[0]> = {}, a: Partial<Page> = {}) => {
  testBoot();
  return renderToString(<PageRow page={{ ...page, ...a }} index={0} sub={null} actions={null} {...p} />);
};

describe(PageRow, () => {
  it('tells no state it has not, and each it has, by its attributes', () => {
    expect.assertions(4);
    const plain = draw();
    expect(plain).not.toMatch(/ (aria-current|data-unread|data-archived|data-kept)=/);
    const all = draw({ picked: true, unread: true, archived: true, kept: true });
    expect(all).toContain('aria-current="true"');
    expect(all).toMatch(/data-unread="true".*data-archived="true".*data-kept="true"/);
    expect(plain).toContain('data-key="entry:a" data-slug="a"');
  });

  it('links its title to its page, with all the row may cut in its title, and marks it read or not', () => {
    expect.assertions(3);
    const html = draw();
    expect(html).toContain('href="/a/a/"');
    expect(html).toContain('title="A page\nwhat it is\nfloe\nui_updated_at:');
    expect([draw().includes('data-testid="ring"'), draw({ unread: true }).includes('data-testid="dot"')]).toStrictEqual(
      [true, true],
    );
  });

  // #530: a folder's time is its link's, which says nothing of what is in it. LIMIT: one draw in node, which takes no
  // way of useMinute; the shape of #530, a time written in as the times are advanced, is not caught here: test
  // it in a DOM run (a browser or jsdom) if the rows come to set their times apart from their draw
  it('shows a folder no time, and a page its time', () => {
    expect.assertions(2);
    const empty = /<time[^>]*><\/time>/;
    expect(draw({}, { folder: true })).toMatch(empty);
    expect(draw()).not.toMatch(empty);
  });

  it('leaves a missing file’s title no link, out of the tab order, and tells a folder’s open', () => {
    expect.assertions(3);
    const gone = draw({}, { missing: true });
    expect(gone).not.toContain('href=');
    expect(gone).toContain('tabindex="-1"');
    expect(draw({ folderOpen: true }, { folder: true })).toContain('aria-expanded="true"');
  });

  it('draws a box to select it where it may be selected, and none where not (a task’s page)', () => {
    expect.assertions(2);
    expect(draw({ onSelect: () => undefined })).toContain('data-testid="sel"');
    expect(draw()).not.toContain('data-testid="sel"');
  });
});
