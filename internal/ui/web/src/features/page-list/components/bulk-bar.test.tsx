import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { BulkBar } from '@/features/page-list/components/bulk-bar';
import { createSelection } from '@/features/page-list/stores/selection';
import { armedAt, armNext } from '@/hooks/use-arm';
import { createStore } from '@/lib/store';
import type { ListQuery, Query } from '@/stores/list-query';
import type { Pages } from '@/stores/pages';

const draw = (selected: string[], view = 'all') => {
  testBoot();
  const selection = createSelection();
  selection.add(selected);
  const query = { query: createStore({ view } as Query) } as unknown as ListQuery;
  const pages = { state: createStore({ entries: [] }) } as unknown as Pages;
  return renderToString(<BulkBar onTags={() => undefined} selection={selection} query={query} pages={pages} />);
};

describe(BulkBar, () => {
  it('draws its buttons hidden with the bar while none is selected, each of its words in a label', () => {
    expect.assertions(3);
    const none = draw([]);
    expect(none).toMatch(/^<section id="bulk" aria-label="[^"]*" hidden=""/);
    expect([...none.matchAll(/<button[^>]* id="(bulk-[a-z]+)"/g)].map((m) => m[1])).toStrictEqual([
      'bulk-page',
      'bulk-tag',
      'bulk-archive',
      'bulk-delete',
      'bulk-clear',
    ]);
    expect([...none.matchAll(/data-testid="label"/g)]).toHaveLength(5);
  });

  it('counts the selected in two words that make its text, and keeps the clearing as the delete waits', () => {
    expect.assertions(2);
    const two = draw(['a', 'b']);
    expect(/<span id="bulk-count"[^>]*>(.*?)<\/span><\/span>/.exec(two)?.[1]?.replace(/<[^>]+>/g, '')).toBe(
      'ui_selected_n:2ui_selected_more:undefined',
    );
    // Hidden as the delete waits: the bar's buttons but the delete and the clearing
    expect(two).toContain('has-[&gt;[data-armed]]:[&amp;&gt;button:not([data-armed],#bulk-clear)]:hidden');
  });

  it('lets the delete’s wait go as the selection changes, the same boxes ticked again too', () => {
    expect.assertions(2);
    const selection = createSelection();
    selection.add(['a']);
    const waits = armNext(null, String(selection.rev.get())).armedFor;
    selection.toggle('a', false);
    selection.toggle('a', true);
    expect(armedAt(waits, String(selection.rev.get()))).toBe(false);
    // Unchanged, it waits on
    expect(armedAt(waits, String(selection.rev.get() - 2))).toBe(true);
  });

  it('archives on any view but the archived, where it restores', () => {
    expect.assertions(1);
    const words = (html: string) => /id="bulk-archive"[^>]*><span data-testid="label">([^<]*)</.exec(html)?.[1];
    expect([words(draw(['a'])), words(draw(['a'], 'archived'))]).toStrictEqual([
      'ui_archive:undefined',
      'ui_unarchive:undefined',
    ]);
  });

  it('lets the selection go by short words stuck at the bar’s end, its name the long words, there as the delete waits', () => {
    expect.assertions(3);
    const html = draw(['a']);
    const clear = String(/<button[^>]* id="bulk-clear"[^>]*>.*?<\/button>/.exec(html)?.[0]);
    expect(clear).toContain('aria-label="ui_clear_selection:undefined"');
    expect(/<span data-testid="label">([^<]*)</.exec(clear)?.[1]).toBe('ui_clear_short:undefined');
    // Stuck at the end; not among those the waiting delete hides
    expect(html).toMatch(
      /\[&amp;&gt;#bulk-clear\]:sticky.*:not\(\[data-armed\],#bulk-clear\)|:not\(\[data-armed\],#bulk-clear\).*\[&amp;&gt;#bulk-clear\]:sticky/,
    );
  });
});
