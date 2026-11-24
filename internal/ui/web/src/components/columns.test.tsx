import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ColumnFoot, Columns, ShelfFoot } from '@/components/columns';
import { testBoot } from '@/components/test-boot';

// The build as committed (just web-build-check holds it to the sources)
import built from '/dist/index.html?raw';
import type { Boot } from '@/config/boot';
import type { Kept } from '@/lib/storage';
import { createColumns } from '@/stores/columns';
import { createOpenPage } from '@/stores/open-page';
import type { Screen } from '@/types/screen';

const memory = (kept: string): Kept => ({
  read: () => kept,
  write: () => undefined,
  remove: () => undefined,
});

function drawn(screen: Screen, kept = '', picked = false, alone = false): string {
  testBoot();
  const openPage = createOpenPage({ boot: () => ({ screen: 'list' }) as unknown as Boot, search: () => '' });
  if (picked) openPage.picked.set({ slug: 'a', file: '', v: 0 });
  const columns = createColumns({ screen: () => screen, storage: memory(kept), openPage });
  return renderToString(
    <Columns
      columns={columns}
      shelf={<nav id="shelf" />}
      list={<ul id="list" />}
      reader={<section id="reader" />}
      alone={alone}
    />,
  );
}

// The opening tag of the element of id, '' for none
const tag = (html: string, id: string): string => new RegExp(`<[^>]* id="${id}"[^>]*>`).exec(html)?.[0] ?? '';
const ids = (html: string): string[] => [...html.matchAll(/ id="([^"]+)"/g)].map((m) => m[1] ?? '');

describe(Columns, () => {
  it('draws each screen’s columns, edges and buttons, and no others', () => {
    expect.assertions(3);
    expect(ids(drawn('list'))).toStrictEqual([
      'manage',
      'shelf-show',
      'list-show',
      'shelf',
      'shelf-grip',
      'listcol',
      'list-hide',
      'list-body',
      'list',
      'list-grip',
      'reader',
    ]);
    expect(ids(drawn('taskList'))).toStrictEqual([
      'manage',
      'shelf-show',
      'shelf',
      'shelf-grip',
      'listcol',
      'list-body',
      'list',
      // There too, hidden (the tests read it on the task list)
      'reader',
    ]);
    expect(ids(drawn('taskPage'))).toStrictEqual([
      'manage',
      'list-show',
      'listcol',
      'list-hide',
      'list-body',
      'list',
      'list-grip',
      'reader',
    ]);
  });

  it('hides the task list’s page beside it, which is there all the same', () => {
    expect.assertions(2);
    expect(drawn('taskList')).toMatch(/<div class="contents" hidden=""><section id="reader">/);
    expect(drawn('list')).toMatch(/<div class="contents"><section id="reader">/);
  });

  it('lays the list’s column out alone, all the rest drawn but hidden', () => {
    expect.assertions(4);
    // The shelf kept folded, as its button would show then
    const html = drawn('taskList', '{"open":false}', false, true);
    expect(ids(html)).toStrictEqual(ids(drawn('taskList')));
    expect(tag(html, 'manage')).toMatch(
      /grid-cols-\[minmax\(0,1fr\)\] grid-rows-\[minmax\(0,1fr\)\] \[&amp;&gt;#shelf\]:hidden/,
    );
    expect(['shelf-show', 'shelf-grip'].map((id) => tag(html, id).includes('hidden=""'))).toStrictEqual([true, true]);
    // A task gone (task-missing): its page and the list's foot and edge hidden too
    const missing = drawn('taskPage', '', true, true);
    expect([
      missing.includes('data-testid="col-foot" hidden=""'),
      tag(missing, 'list-grip').includes('hidden=""'),
      missing.includes('<div class="contents" hidden=""><section id="reader">'),
    ]).toStrictEqual([true, true, true]);
  });

  it('puts the hidden heading first, and the list’s foot in the list’s column', () => {
    expect.assertions(2);
    const html = drawn('list');
    expect(html).toMatch(/^<main[^>]*><h1 class="sr-only">floe<\/h1>/);
    expect(html).toMatch(
      /<section[^>]* id="listcol"[^>]*><div data-testid="col-foot"[^>]*><button[^>]* id="list-hide"/,
    );
  });

  it('tells what each button folds and opens, and hides those that have nothing to do', () => {
    expect.assertions(4);
    const open = drawn('list');
    expect(tag(open, 'list-hide')).toMatch(/aria-expanded="true" aria-controls="listcol"/);
    expect(tag(open, 'list-show')).toMatch(/hidden="".*aria-expanded="false" aria-controls="listcol"/);
    expect(tag(open, 'shelf-show')).toMatch(/hidden="".*aria-controls="shelf"/);
    const folded = drawn('list', '{"open":false,"listOpen":false}');
    expect(
      [tag(folded, 'shelf-show'), tag(folded, 'list-show'), tag(folded, 'shelf-grip'), tag(folded, 'list-grip')].map(
        (t) => t.includes('hidden=""'),
      ),
    ).toStrictEqual([false, false, true, true]);
  });

  it('keeps a task page’s rows, with no button to fold them, until a page is picked', () => {
    expect.assertions(2);
    expect(tag(drawn('taskPage', '{"listOpen":false}'), 'list-hide')).toContain('hidden=""');
    expect(tag(drawn('taskPage', '{"listOpen":false}', true), 'listcol')).toContain('not-wide:hidden');
  });

  it('prints neither button that opens a column, which lie over the words', () => {
    expect.assertions(1);
    const html = drawn('list', '{"open":false,"listOpen":false}');
    expect(['shelf-show', 'list-show'].map((id) => tag(html, id).includes('print:hidden'))).toStrictEqual([true, true]);
  });

  it('raises what opens the shelf only with the list folded too, where it lies over the page', () => {
    expect.assertions(2);
    const raised = (html: string) => tag(html, 'shelf-show').includes('bg-panel');
    expect(raised(drawn('list', '{"open":false}'))).toBe(false);
    expect(raised(drawn('list', '{"open":false,"listOpen":false}'))).toBe(true);
  });

  it('folds the shelf by the grid’s own class, and keeps the edges out of its cells', () => {
    expect.assertions(3);
    expect(tag(drawn('list', '{"open":false}'), 'manage')).toContain('not-wide:[&amp;&gt;#shelf]:hidden');
    expect(tag(drawn('list'), 'manage')).not.toContain('#shelf');
    expect(drawn('list')).toMatch(/<div class="contents wide:hidden"><div[^>]* id="shelf-grip"/);
  });
});

describe(ShelfFoot, () => {
  it('holds the button that folds the shelf', () => {
    expect.assertions(1);
    testBoot();
    expect(renderToString(<ShelfFoot />)).toMatch(
      /^<div data-testid="col-foot"[^>]*><button[^>]* id="shelf-hide"[^>]*aria-expanded="true" aria-controls="shelf"/,
    );
  });
});

describe(ColumnFoot, () => {
  it('goes under 70rem, but where its column folds at every width', () => {
    expect.assertions(2);
    expect(renderToString(<ColumnFoot />)).toContain('wide:hidden');
    expect(renderToString(<ColumnFoot always />)).not.toContain('wide:hidden');
  });

  // Tailwind makes a rule for a class only where its source reads it as a word of its own: `pe-foot-pad${…}` was none.
  // Failing, rebuild the dist (just web-build); failing still, look for a class written with no space before a `${`
  // LIMIT: sees the classes of word characters alone (not wide:hidden), and reads `.<class>{` in the minified CSS: a
  // rule shared as `.a,.b{` fails it though there. Read the CSS's selectors if that comes
  it('has a rule in the build for each of its plain classes', () => {
    expect.assertions(1);
    const classes = [<ColumnFoot key="a" />, <ColumnFoot key="b" always next />]
      .flatMap((foot) => String(/class="([^"]*)"/.exec(renderToString(foot))?.[1]).split(' '))
      .filter((c) => /^[\w-]+$/.test(c));
    expect(classes.filter((c) => !built.includes(`.${c}{`))).toStrictEqual([]);
  });
});
