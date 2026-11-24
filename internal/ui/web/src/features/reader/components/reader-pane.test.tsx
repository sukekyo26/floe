import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { FrameHead } from '@/features/reader/components/frame-head';
import { NotFound } from '@/features/reader/components/not-found';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import type { PageReads } from '@/features/reader/stores/page-reads';
import type { Page } from '@/types/page';

const page = (change: Partial<Page> = {}): Page => ({
  slug: 'a',
  title: 'A',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...change,
});
const reads = { opened: () => undefined, served: () => Promise.resolve() } as PageReads;
const pane = (props: Partial<Parameters<typeof ReaderPane>[0]>) => {
  testBoot();
  return renderToString(
    <ReaderPane at={null} page={null} keys={new Set()} onKey={() => undefined} reads={reads} {...props} />,
  );
};
const tag = (html: string, start: string) => String(new RegExp(`${start}[^>]*>`).exec(html)?.[0]);

describe(ReaderPane, () => {
  it('shows the page chosen in its iframe, named and marked by what is open', () => {
    expect.assertions(3);
    const html = pane({ at: { slug: 'a', file: '', v: 2 }, page: page() });
    expect(tag(html, '<iframe')).toMatch(
      /id="read-frame" data-loaded="about:blank" title="A" data-slug="a" data-version="2" allow="clipboard-write \*; fullscreen \*"(?![^>]*hidden)/,
    );
    expect(tag(html, '<iframe')).not.toMatch(/sandbox|data-file/);
    expect(tag(html, '<p id="read-blank"')).toContain('hidden=""');
  });

  it('says none is chosen, or that a link’s file is gone, with no iframe shown and the head hidden', () => {
    expect.assertions(3);
    const none = pane({});
    expect([tag(none, '<div id="read-head"'), tag(none, '<iframe')].map((x) => x.includes('hidden=""'))).toStrictEqual([
      true,
      true,
    ]);
    expect(none).toMatch(/id="read-blank"[^>]*>ui_read_blank/);
    expect(pane({ at: { slug: 'a', file: '', v: 0 }, page: page({ missing: true, path: '/tmp/a.html' }) })).toMatch(
      /id="read-blank"[^>]*>ui_link_missing:\/tmp\/a.html/,
    );
  });

  it('puts the words of the version shown over those of a page newly at another version, under the tags', () => {
    expect.assertions(1);
    const html = pane({ tags: <i id="t" />, versionNote: <b id="v" /> });
    expect(['id="t"', 'id="v"', '<iframe'].map((x) => html.indexOf(x))).toStrictEqual(
      ['id="t"', 'id="v"', '<iframe'].map((x) => html.indexOf(x)).toSorted((a, b) => a - b),
    );
  });

  it('puts the bar of the page shown on the right, of its own id alone, and none with no page', () => {
    expect.assertions(2);
    const bar = <span id="read-actions">x</span>;
    expect(pane({ at: { slug: 'a', file: '', v: 0 }, page: page(), actions: bar })).toMatch(
      /<span class="ms-auto"><span id="read-actions">x<\/span><\/span>/,
    );
    expect(pane({ actions: bar })).not.toContain('read-actions');
  });

  it('shows the head with the list’s narrowing in words though no page is chosen, a folder’s file by its name', () => {
    expect.assertions(2);
    expect(tag(pane({ what: <span id="shelf-what">x</span> }), '<div id="read-head"')).not.toContain('hidden=""');
    const file = pane({ at: { slug: 'a', file: 'docs/b.html', v: 0 }, page: page({ folder: true }) });
    expect(tag(file, '<iframe')).toMatch(/title="docs\/b.html" data-slug="a" data-file="docs\/b.html"/);
  });
});

describe(FrameHead, () => {
  it('names a folder’s file under its title, and marks an archived page', () => {
    expect.assertions(2);
    testBoot();
    const html = renderToString(<FrameHead page={page({ archived: true })} file="docs/b.html" />);
    expect(html).toMatch(/id="frame-title"[^>]*><span class="crumb[^"]*">A \/ <\/span>docs\/b.html/);
    expect(tag(html, '<span id="frame-archived"')).not.toContain('hidden');
  });
});

describe(NotFound, () => {
  it('says a link’s file gone or no page of the slug, the way back a link of the button’s shape', () => {
    expect.assertions(2);
    testBoot();
    expect(renderToString(<NotFound page={null} slug="q" back="/?view=fav" hidden={false} />)).toMatch(
      /ui_not_found_detail:q.*<a id="notfound-back"[^>]*href="\/\?view=fav"/s,
    );
    expect(
      renderToString(<NotFound page={page({ missing: true, path: '/tmp/a' })} slug="a" back="/" hidden={false} />),
    ).toContain('ui_link_missing:/tmp/a');
  });
});
