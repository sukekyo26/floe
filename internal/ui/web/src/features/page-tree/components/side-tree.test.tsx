import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { SideTree } from '@/features/page-tree/components/side-tree';
import type { Kept } from '@/lib/storage';
import { createKeptSide } from '@/stores/kept-side';
import { createFolders } from '@/stores/open-folders';
import type { Listing, Page } from '@/types/page';

const memory = (kept: string): Kept => ({
  read: () => kept,
  write: () => undefined,
  remove: () => undefined,
});

const docs: Page = {
  slug: 'docs',
  title: 'Docs anew',
  folder: true,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
};

const listing = (dirs: string[], files: string[]): Listing => ({
  dirs,
  files: files.map((file) => ({ file, updatedAt: '' })),
  more: 0,
});

function drawn(folder: Page | null, kept = ''): string {
  testBoot();
  const folders = createFolders({ storage: memory('') });
  folders.side.set({
    opened: new Set(['', 'api', 'guide']),
    listings: new Map<string, Listing | Error>([
      ['', listing(['api', 'guide'], ['index.html'])],
      ['guide', listing([], ['guide/setup.html', 'guide/start.html'])],
      ['api', new Error('no')],
    ]),
  });
  const side = createKeptSide('frame.side', 280, { storage: memory(kept) });
  return renderToString(<SideTree folder={folder} file="guide/setup.html" folders={folders} side={side} />);
}

// The opening tag of the element of id, '' for none
const tag = (html: string, id: string): string => new RegExp(`<[^>]* id="${id}"[^>]*>`).exec(html)?.[0] ?? '';

// The tree's rows as drawn: each child's element, title, aria-expanded, aria-current and href
function treeRows(html: string): string[] {
  const tree = /<div id="side-tree"[^>]*>(.*?)<\/div><\/nav>/.exec(html)?.[1] ?? '';
  return [...tree.matchAll(/<(button|a|p) ([^>]*)>/g)].map(([, el, attrs]) => {
    const at = (name: string) => new RegExp(` ?${name}="([^"]*)"`).exec(` ${attrs ?? ''}`)?.[1];
    return [el, at('title'), at('aria-expanded'), at('aria-current'), at('href')].filter(Boolean).join(' ');
  });
}

describe(SideTree, () => {
  it('names itself by the folder’s title of now, in its head and its label', () => {
    expect.assertions(2);
    const html = drawn(docs);
    expect(tag(html, 'side')).toContain('aria-label="Docs anew"');
    expect(html).toMatch(/ id="side-title"[^>]*>Docs anew</);
  });

  it('draws its rows as the tree’s own children: directories that open, pages that the frame moves to', () => {
    expect.assertions(1);
    const rows = treeRows(drawn(docs));
    expect(rows).toStrictEqual([
      'button api true',
      'p',
      'button guide true',
      'a guide/setup.html page /a/docs/?file=guide%2Fsetup.html',
      'a guide/start.html /a/docs/?file=guide%2Fstart.html',
      'a index.html /a/docs/?file=index.html',
    ]);
  });

  it('hides the tree, its edge and what opens it while the folder is not found', () => {
    expect.assertions(1);
    const html = drawn(null);
    expect(['side', 'side-grip', 'side-show'].map((id) => tag(html, id).includes('hidden=""'))).toStrictEqual([
      true,
      true,
      true,
    ]);
  });

  it('folds as kept, what opens it then shown, raised and not printed; its foot at every width', () => {
    expect.assertions(3);
    const html = drawn(docs, '{"width":300,"open":false}');
    expect([tag(html, 'side'), tag(html, 'side-grip')].map((t) => t.includes('hidden=""'))).toStrictEqual([true, true]);
    expect(['hidden=""', 'bg-panel', 'print:hidden'].map((c) => tag(html, 'side-show').includes(c))).toStrictEqual([
      false,
      true,
      true,
    ]);
    expect(html).toMatch(/<div data-testid="col-foot" class="(?![^"]*wide:hidden)[^"]*"><button[^>]* id="side-hide"/);
  });

  it('rings its buttons and rows inside them, and leaves the tree and its edge out of print', () => {
    expect.assertions(2);
    const html = drawn(docs);
    expect(tag(html, 'side')).toContain('[&amp;_:is(a,button):focus-visible]:-outline-offset-2');
    expect([tag(html, 'side'), tag(html, 'side-grip')].map((t) => t.includes('print:hidden'))).toStrictEqual([
      true,
      true,
    ]);
  });
});
