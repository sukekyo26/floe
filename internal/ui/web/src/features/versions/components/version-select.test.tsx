import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { getBoot } from '@/config/boot';
import { VersionNote } from '@/features/versions/components/version-note';
import { VersionSelect } from '@/features/versions/components/version-select';
import { createVersions } from '@/features/versions/stores/versions';
import type { Client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { createOpenPage, type OpenAt } from '@/stores/open-page';
import type { Reads } from '@/stores/reads';
import type { Page } from '@/types/page';

const page = (slug = 'a'): Page => ({ slug, version: 3, versionCount: 3 }) as Page;
const at = (v = 0, slug = 'a'): OpenAt => ({ slug, file: '', v });
const AT = '2026-11-01T09:00:00Z';

// The versions of a, 1 to 3, read for at beside the list, or whole at a version asked for gone
async function read(x: OpenAt, gone = false) {
  testBoot();
  const openPage = createOpenPage({ boot: getBoot, search: () => '' });
  const client = {
    api: () =>
      Promise.resolve(new Response(JSON.stringify({ versions: [1, 2, 3].map((n) => ({ n, at: AT })), read: 2 }))),
  } as unknown as Client;
  const reads = { keptUnread: createStore<string | null>(null), answered: createStore(null) } as unknown as Reads;
  const versions = createVersions({
    client,
    openPage,
    reads,
    boot: () => (gone ? { ...getBoot(), screen: 'frame', framed: 'a', version: 'gone' } : getBoot()),
  });
  if (gone) versions.arrive();
  openPage.picked.set(x);
  versions.want(x, page());
  await new Promise((go) => setTimeout(go, 0));
  return versions;
}

describe(VersionSelect, () => {
  it('lists the versions newest first, the latest chosen, the number alone closed', async () => {
    expect.assertions(2);
    const versions = await read(at());
    const html = renderToString(<VersionSelect at={at()} page={page()} versions={versions} />);
    expect(html).toMatch(
      /^<select id="read-version" aria-label="ui_version:undefined" data-version-pick="" data-numbers="1,2,3" class="[^"]*"><button type="button"><selectedcontent><\/selectedcontent><\/button>/,
    );
    expect(
      [
        ...html.matchAll(/<option value="(\d)"[^>]*><span>v\d<\/span><span class="in-\[selectedcontent\]:hidden"> · /g),
      ].map((m) => m[1]),
    ).toStrictEqual(['3', '2', '1']);
  });

  it('marks a version other than the latest, the frame by its own id, and hides itself with none read', async () => {
    expect.assertions(3);
    const versions = await read(at(1));
    expect(renderToString(<VersionSelect at={at(1)} page={page()} framed versions={versions} />)).toMatch(
      /^<select id="frame-version"[^>]* data-old="true"/,
    );
    // Another page's versions are not drawn: hidden, with no options
    const other = renderToString(<VersionSelect at={at(0, 'b')} page={page('b')} versions={versions} />);
    expect(other).toMatch(/^<select [^>]*hidden=""/);
    expect(other).not.toContain('<option');
  });
});

describe(VersionNote, () => {
  it('tells the version shown, with the ways to the latest and to the diff', async () => {
    expect.assertions(2);
    const versions = await read(at(1));
    const html = renderToString(<VersionNote at={at(1)} page={page()} versions={versions} />);
    expect(html).toMatch(/^<output data-testid="ver-note"/);
    expect([...html.matchAll(/<a href="([^"]+)"/g)].map((m) => m[1])).toStrictEqual([
      '/a/a/',
      '/a/a/?diff&amp;from=1&amp;to=3',
    ]);
  });

  it('tells a version asked for gone, and nothing of the latest', async () => {
    expect.assertions(2);
    const gone = renderToString(<VersionNote at={at()} page={page()} versions={await read(at(), true)} />);
    expect(gone).toMatch(/^<output data-testid="ver-note"[^>]*>ui_version_gone/);
    expect(renderToString(<VersionNote at={at()} page={page()} versions={await read(at())} />)).toBe('');
  });
});
