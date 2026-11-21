import { describe, expect, it, vi } from 'vitest';

import { createClient } from '@/lib/api-client';

// The page's window is the global one, as in a browser, so that window.fetch is the fetch a test stubs
vi.stubGlobal('window', globalThis);

// A fetch whose writes' answers hold their bodies until body settles; a GET answers at once
const holdingWrites = (body: Promise<string>) => (_url: string, init?: RequestInit) =>
  Promise.resolve(
    init?.method && init.method !== 'GET'
      ? ({ status: 200, statusText: '', headers: new Headers(), text: () => body } as unknown as Response)
      : new Response('list'),
  );

// A fetch that answers with each of answers in turn
function inTurn(answers: Response[]) {
  let next = 0;
  return () => Promise.resolve(answers[next++] ?? new Response('no more answers', { status: 599 }));
}

describe(createClient, () => {
  it('looks window.fetch up at each call, so that one replaced after the page loaded is the one asked', async () => {
    expect.assertions(1);
    const { api } = createClient();
    const asked: string[] = [];
    vi.stubGlobal('fetch', () => {
      asked.push('first');
      return Promise.resolve(new Response('a'));
    });
    await api('/x');
    vi.stubGlobal('fetch', () => {
      asked.push('second');
      return Promise.resolve(new Response('b'));
    });
    await api('/x');
    expect(asked).toStrictEqual(['first', 'second']);
  });

  it('tells the server reached by an answer, an HTTP error too, down by a throw, and nothing by a time limit or a call off', async () => {
    expect.assertions(5);
    const { api, reached } = createClient();
    vi.stubGlobal('fetch', () => Promise.resolve(new Response('no', { status: 500 })));
    await api('/x');
    vi.stubGlobal('fetch', () => Promise.reject(new DOMException('slow', 'TimeoutError')));
    await expect(api('/x')).rejects.toThrow('slow');
    vi.stubGlobal('fetch', () => Promise.reject(new DOMException('off', 'AbortError')));
    await expect(api('/x')).rejects.toThrow('off');
    const kept = reached.get();
    vi.stubGlobal('fetch', () => Promise.reject(new TypeError('Failed to fetch')));
    await expect(api('/x')).rejects.toThrow('Failed to fetch');
    expect(kept).toBe(true);
    expect(reached.get()).toBe(false);
  });

  it('holds noWrites until every write has read its answer, and counts no GET', async () => {
    expect.assertions(4);
    const { api, noWrites, writesSent } = createClient();
    const body = Promise.withResolvers<string>();
    vi.stubGlobal('fetch', holdingWrites(body.promise));
    await api('/x');
    expect(noWrites()).toBeNull();
    const written = api('/x', { method: 'PATCH' });
    const waits = noWrites();
    let free = false;
    void waits?.then(() => {
      free = true;
    });
    await new Promise((go) => {
      setTimeout(go, 1);
    });
    expect(free).toBe(false);
    body.resolve('done');
    await written;
    await waits;
    expect(free).toBe(true);
    expect(writesSent()).toBe(1);
  });

  it('makes the answer of a write with no body anew with none', async () => {
    expect.assertions(1);
    const { api } = createClient();
    vi.stubGlobal('fetch', inTurn([204, 205, 304].map((status) => new Response(null, { status }))));
    const got = [];
    for (let i = 0; i < 3; i++) got.push((await api('/x', { method: 'DELETE' })).status);
    expect(got).toStrictEqual([204, 205, 304]);
  });
});
