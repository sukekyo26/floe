import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createClient } from '@/lib/api-client';
import { readListing, sendPage } from '@/lib/pages';

vi.stubGlobal('window', globalThis);

const answering = (res: Response) => () => Promise.resolve(res);

describe(readListing, () => {
  it('reads a directory, null for one not there, and an Error for another refusal', async () => {
    expect.assertions(3);
    setBoot({ messages: { ui_http_error: '%s (%d)' } } as unknown as Boot);
    const client = createClient();
    vi.stubGlobal('fetch', answering(new Response('{"dirs":["x"],"files":[],"more":0}')));
    await expect(readListing(client, 'a', '')).resolves.toStrictEqual({ dirs: ['x'], files: [], more: 0 });
    vi.stubGlobal('fetch', answering(new Response('', { status: 404 })));
    await expect(readListing(client, 'a', '')).resolves.toBeNull();
    vi.stubGlobal('fetch', answering(new Response('locked', { status: 500 })));
    await expect(readListing(client, 'a', '')).resolves.toStrictEqual(new Error('locked (500)'));
  });
});

describe(sendPage, () => {
  it('tells the reason of a refusal, a 404 none when gone counts, and the answer when asked', async () => {
    expect.assertions(3);
    setBoot({ messages: { ui_http_error: '%s (%d)' } } as unknown as Boot);
    const client = createClient();
    vi.stubGlobal('fetch', answering(new Response('no', { status: 404 })));
    await expect(sendPage(client, 'a', { method: 'DELETE' })).resolves.toStrictEqual({
      reason: 'no (404)',
      answer: null,
    });
    vi.stubGlobal('fetch', answering(new Response('', { status: 404 })));
    await expect(sendPage(client, 'a', { method: 'DELETE' }, { gone: true })).resolves.toStrictEqual({
      reason: '',
      answer: null,
    });
    vi.stubGlobal('fetch', answering(new Response('{"slug":"a"}')));
    await expect(sendPage(client, 'a', { method: 'PATCH' }, { answer: true })).resolves.toStrictEqual({
      reason: '',
      answer: { slug: 'a' },
    });
  });
});
