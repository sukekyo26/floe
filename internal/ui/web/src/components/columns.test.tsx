import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { Columns, ShelfFoot } from '@/components/columns';
import { testBoot } from '@/components/test-boot';
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

function drawn(screen: Screen, kept = '', picked = false): string {
  testBoot();
  const openPage = createOpenPage({ boot: () => ({ screen: 'list' }) as unknown as Boot, search: () => '' });
  if (picked) openPage.picked.set({ slug: 'a', file: '', v: 0 });
  const columns = createColumns({ screen: () => screen, storage: memory(kept), openPage });
  return renderToString(
    <Columns columns={columns} shelf={<nav id="shelf" />} list={<ul id="list" />} reader={<section id="reader" />} />,
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
