import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { IconButton } from '@/components/icon-button';

describe(IconButton, () => {
  it('takes the classes of one size alone', () => {
    expect.assertions(4);
    const row = renderToString(<IconButton size="row">x</IconButton>);
    expect(row).toContain('h-6.5');
    expect(row).not.toContain('h-icon-btn');
    const bar = renderToString(<IconButton>x</IconButton>);
    expect(bar).toContain('h-icon-btn');
    expect(bar).not.toContain('h-6.5');
  });

  it('takes the gap of one size alone, a menu item’s wider', () => {
    expect.assertions(2);
    const gaps = (html: string) =>
      String(/class="([^"]*)"/.exec(html)?.[1])
        .split(' ')
        .filter((c) => c.startsWith('gap-'));
    expect(gaps(renderToString(<IconButton size="item">x</IconButton>))).toStrictEqual(['gap-2.5']);
    expect(gaps(renderToString(<IconButton>x</IconButton>))).toStrictEqual(['gap-1.5']);
  });

  it('tells a toggle pressed or not, a button that toggles nothing neither, and Restore on but not pressed', () => {
    expect.assertions(3);
    expect(renderToString(<IconButton pressed={false}>x</IconButton>)).toContain('aria-pressed="false"');
    expect(renderToString(<IconButton>x</IconButton>)).not.toContain(' aria-pressed=');
    expect(renderToString(<IconButton on>x</IconButton>)).toMatch(/data-on="true"(?![^>]* aria-pressed=)/);
  });

  it('stands on a ground of its own with a shadow raised, transparent otherwise', () => {
    expect.assertions(2);
    const classes = (html: string) =>
      String(/class="([^"]*)"/.exec(html)?.[1])
        .split(' ')
        .filter((c) => /^(bg-|shadow|ring)/.test(c));
    expect(classes(renderToString(<IconButton raised>x</IconButton>))).toStrictEqual([
      'bg-panel',
      'shadow-sm',
      'ring-1',
      'ring-line',
    ]);
    expect(classes(renderToString(<IconButton>x</IconButton>))).toStrictEqual(['bg-transparent']);
  });

  it('leaves a button out of print only where asked', () => {
    expect.assertions(2);
    expect(renderToString(<IconButton display="screen">x</IconButton>)).toContain('print:hidden');
    expect(renderToString(<IconButton>x</IconButton>)).not.toContain('print:hidden');
  });
});
