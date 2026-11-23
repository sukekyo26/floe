import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { FramePage } from '@/features/reader/components/frame-page';

const draw = (found: boolean, diff = false) => {
  vi.stubGlobal('window', { location: { hash: '#part', href: 'http://127.0.0.1:7325/a/p/?v=2' } });
  return renderToString(
    <FramePage
      at={{ slug: 'p', file: '', v: 2 }}
      found={found}
      diff={diff}
      name="Notes"
      keys={new Set()}
      onKey={() => undefined}
    />,
  );
};

describe(FramePage, () => {
  it('loads the page at its version with the URL’s fragment once found, sandboxed by the server alone', () => {
    expect.assertions(3);
    const html = draw(true);
    expect(html).toMatch(
      /^<iframe id="frame" title="Notes" src="\/a\/p\/\?raw&amp;v=2#part" data-slug="p" data-version="2"/,
    );
    expect(html).toContain('allow="clipboard-write *; fullscreen *"');
    expect(html).not.toMatch(/sandbox=|hidden=""/);
  });

  it('is there, hidden and loading nothing, till found, and in a diff, which reads nothing of the latest', () => {
    expect.assertions(2);
    expect(draw(false)).toMatch(/^<iframe id="frame"(?![^>]*src=)[^>]*hidden=""/);
    expect(draw(true, true)).toMatch(/^<iframe id="frame"(?![^>]*src=)[^>]*hidden=""/);
  });
});
