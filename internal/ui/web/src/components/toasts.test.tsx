import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { Toasts } from '@/components/toasts';
import { createToasts } from '@/stores/toasts';

describe(Toasts, () => {
  it('marks a toast timed and a page’s apart, and pads one with a button by one padding alone', () => {
    expect.assertions(4);
    vi.useFakeTimers();
    const store = createToasts();
    store.toast('both', { ms: 8000, page: true, button: <button type="button">undo</button> });
    store.toast('plain');
    const html = renderToString(<Toasts store={store} />);
    vi.useRealTimers();
    expect(html).toContain('id="toasts" aria-live="polite"');
    expect(html).toMatch(/data-testid="toast" data-timed="true" data-page="true"/);
    expect(html).toMatch(/data-timed[^>]*py-1\.5(?![^"]*py-2\.5)/);
    expect(html).toMatch(/data-testid="toast" class="[^"]*py-2\.5(?![^"]*py-1\.5)/);
  });
});
