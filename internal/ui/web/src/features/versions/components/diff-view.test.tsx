import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { DiffCount } from '@/features/versions/components/diff-count';
import { DiffPicks } from '@/features/versions/components/diff-picks';
import { DiffToc } from '@/features/versions/components/diff-toc';
import { DiffView } from '@/features/versions/components/diff-view';
import { createDiff } from '@/features/versions/stores/diff';
import type { Section } from '@/features/versions/utils/diff-words';
import type { Client } from '@/lib/api-client';
import type { Kept } from '@/lib/storage';
import { createKeptSide } from '@/stores/kept-side';
import type { Page } from '@/types/page';

const none: Kept = { read: () => null, write: () => undefined, remove: () => undefined };
const toc = () => createKeptSide('diff.toc', 240, { storage: none });
const section = (state: Section['state'], text: string, place?: number): Section => ({
  state,
  level: 2,
  heading: [{ op: '', text }],
  ...(place === undefined ? {} : { place }),
});

describe(DiffToc, () => {
  it('draws a section to go to as a button, one with no place as words, the one gone to marked', () => {
    expect.assertions(3);
    testBoot();
    const html = renderToString(
      <DiffToc
        sections={[section('changed', 'A', 0), section('same', 'B')]}
        section={0}
        onPlace={() => undefined}
        width={240}
        innerWidth={1200}
        toc={toc()}
      />,
    );
    const rows = String(/<div id="diff-toc"[^>]*>(.*?)<\/div><p/.exec(html)?.[1]);
    expect(rows).toMatch(
      /^<div class="[^"]*"><button type="button" data-section="0" data-on="true"[^>]*><span>A<\/span><span class="[^"]*"><span aria-hidden="true">ui_diff_mark_changed:undefined<\/span>/,
    );
    expect(rows).toMatch(/<div class="[^"]*"><div class="[^"]*"><span>B<\/span><\/div><\/div>$/);
    // Folded or not, the button that opens it is there, hidden while it shows
    expect(html).toMatch(/<button[^>]*id="diff-toc-show"[^>]*hidden=""/);
  });

  it('keeps its foot at every width it shows at, and leaves what opens it out of print', () => {
    expect.assertions(2);
    testBoot();
    const html = renderToString(
      <DiffToc sections={[]} section={-1} onPlace={() => undefined} width={240} innerWidth={1200} toc={toc()} />,
    );
    expect(/<div data-testid="col-foot" class="([^"]*)"/.exec(html)?.[1]).toMatch(
      /^(?!.*wide:hidden)(?=.*\bpe-foot-pad\b).*ps-foot-pad/,
    );
    expect(/<button[^>]*id="diff-toc-show"[^>]*class="([^"]*)"/.exec(html)?.[1]).toContain('print:hidden');
  });
});

describe(DiffCount, () => {
  it('counts in a span each, all the same in one sentence', () => {
    expect.assertions(2);
    testBoot();
    expect(renderToString(<DiffCount counts={{ changed: 2, added: 0, removed: 1, same: 3 }} />)).toMatch(
      /^<output id="diff-count"[^>]*><span><b[^>]*>2<\/b> ui_diff_changed_n:undefined<\/span><span><b[^>]*>1<\/b>/,
    );
    expect(renderToString(<DiffCount counts={{ changed: 0, added: 0, removed: 0, same: 3 }} />)).toContain(
      'ui_diff_nothing',
    );
  });
});

describe(DiffPicks, () => {
  it('names the two choices and leaves out of each what would not be older or later than the other', () => {
    expect.assertions(2);
    testBoot();
    const list = { versions: [1, 2, 3].map((n) => ({ n, at: '2026-11-01T09:00:00Z' })), read: 0 };
    const html = renderToString(
      <DiffPicks slug="a" list={list} pair={{ from: 1, to: 2 }} onChoose={() => undefined} />,
    );
    expect([...html.matchAll(/<select id="(diff-\w+)"/g)].map((m) => m[1])).toStrictEqual(['diff-from', 'diff-to']);
    expect(
      [...html.matchAll(/<option value="(\d)"( disabled="")?/g)].map((m) => [m[1], m[2] === undefined]),
    ).toStrictEqual([
      ['3', false],
      ['2', false],
      ['1', true],
      ['3', true],
      ['2', true],
      ['1', false],
    ]);
  });
});

describe(DiffView, () => {
  it('keeps its head, columns and iframe with nothing to compare, hidden, and writes no page as it is drawn', () => {
    expect.assertions(2);
    testBoot();
    const diff = createDiff({ client: {} as Client, search: () => '', replace: () => undefined });
    diff.open({ slug: 'a', versionCount: 1 } as Page);
    const html = renderToString(
      <DiffView
        page={{ slug: 'a' } as Page}
        hidden={false}
        keys={new Set()}
        onKey={() => undefined}
        diff={diff}
        toc={toc()}
      />,
    );
    expect(
      ['diff-head', 'diff-main', 'diff-frame', 'diff-none'].map(
        (id) => new RegExp(`id="${id}"[^>]*?( hidden="")?[ />]`).exec(html)?.[1] === undefined,
      ),
    ).toStrictEqual([false, false, true, true]);
    expect(html).not.toMatch(/<iframe[^>]* src=/);
  });

  it("tells the contents' width within bounds on their grip folded too, the columns giving them none", () => {
    expect.assertions(2);
    testBoot();
    const folded: Kept = { read: () => '{"width":240,"open":false}', write: () => undefined, remove: () => undefined };
    const diff = createDiff({ client: {} as Client, search: () => '', replace: () => undefined });
    const html = renderToString(
      <DiffView
        page={null}
        hidden={false}
        keys={new Set()}
        onKey={() => undefined}
        diff={diff}
        toc={createKeptSide('diff.toc', 240, { storage: folded })}
      />,
    );
    // Drawn to a string the window is 0 wide: the bounds are 160 alone
    expect(/id="diff-toc-grip"[^>]*aria-valuenow="(\d+)"/.exec(html)?.[1]).toBe('160');
    expect(html).toContain('--toc:0px');
  });
});
