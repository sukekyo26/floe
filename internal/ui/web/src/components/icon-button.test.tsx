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

  it('tells a toggle pressed or not, a button that toggles nothing neither, and Restore on but not pressed', () => {
    expect.assertions(3);
    expect(renderToString(<IconButton pressed={false}>x</IconButton>)).toContain('aria-pressed="false"');
    expect(renderToString(<IconButton>x</IconButton>)).not.toContain(' aria-pressed=');
    expect(renderToString(<IconButton on>x</IconButton>)).toMatch(/data-on=""(?![^>]* aria-pressed=)/);
  });
});
