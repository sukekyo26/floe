import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { Toasts } from '@/components/toasts';
import { createToasts } from '@/stores/toasts';

const press = () => undefined;

describe(Toasts, () => {
  it('marks a toast timed and a page’s apart, and pads one with a button by one padding alone', () => {
    expect.assertions(3);
    vi.useFakeTimers();
    const store = createToasts();
    store.toast('both', { ms: 8000, page: true, button: { label: 'undo', undo: true, holds: true, onPress: press } });
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

  it('draws a link in place of a button as the link the tests know, the toast taking no pointer', () => {
    expect.assertions(2);
    vi.useFakeTimers();
    const store = createToasts();
    store.toast('', { ms: 8000, button: { label: 'Task #1 made', href: '/tasks?project=p&task=%231' } });
    const html = renderToString(<Toasts store={store} />);
    vi.useRealTimers();
    expect(html).toMatch(
      /<a data-testid="toast-link" href="\/tasks\?project=p&amp;task=%231" class="underline [^"]*pointer-events-auto[^"]*">Task #1 made<\/a>/,
    );
    expect(html).toMatch(/data-testid="toast"[^>]*class="[^"]*py-2.5[^"]*"(?![^>]*pointer-events-auto)/);
  });

  it('marks a way back undo with its page’s slug, and any other button neither', () => {
    expect.assertions(2);
    vi.useFakeTimers();
    const store = createToasts();
    store.toast('archived', {
      ms: 8000,
      button: { label: 'Restore', name: 'Restore a', slug: 'a', undo: true, holds: false, onPress: press },
    });
    store.toast('read', { ms: 8000, button: { label: 'Mark as unread', undo: false, holds: false, onPress: press } });
    const html = renderToString(<Toasts store={store} />);
    vi.useRealTimers();
    expect(html).toMatch(/<button[^>]*data-testid="undo"[^>]*data-slug="a"[^>]*aria-label="Restore a"/);
    expect(html.match(/data-testid="undo"/g)).toHaveLength(1);
  });

  it('names each toast by its id, for its button to be found once a modal over it closes', () => {
    expect.assertions(1);
    vi.useFakeTimers();
    const store = createToasts();
    const id = store.toast('off', { ms: 8000, button: { label: 'undo', undo: true, holds: true, onPress: press } });
    const html = renderToString(<Toasts store={store} />);
    vi.useRealTimers();
    expect(html).toContain(`data-toast="${String(id)}"`);
  });
});
