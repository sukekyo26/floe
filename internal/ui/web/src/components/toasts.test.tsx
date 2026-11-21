import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { Toasts } from '@/components/toasts';
import { createToasts } from '@/stores/toasts';

describe(Toasts, () => {
  it('marks a toast timed and a page’s apart, and pads one with a button by one padding alone', () => {
    expect.assertions(3);
    vi.useFakeTimers();
    const store = createToasts();
    store.toast('both', { ms: 8000, page: true, button: <button type="button">undo</button> });
    store.toast('plain');
    const html = renderToString(<Toasts store={store} />);
    vi.useRealTimers();
    expect(html).toContain('id="toasts" aria-live="polite"');
    expect(html).toMatch(/data-testid="toast" data-timed="true" data-page="true"/);
    const classes = [...html.matchAll(/data-testid="toast"[^>]*class="([^"]*)"/g)].map((m) => String(m[1]).split(' '));
    expect(classes.map((c) => [c.includes('py-1.5'), c.includes('py-2.5')])).toStrictEqual([
      [true, false],
      [false, true],
    ]);
  });
});
