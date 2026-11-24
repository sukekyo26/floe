import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createClient } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { createNotice } from '@/stores/notice';
import { createOpenPage } from '@/stores/open-page';
import type { Pages } from '@/stores/pages';
import { createReads } from '@/stores/reads';

vi.stubGlobal('window', globalThis);

const BOOT = {
  screen: 'list',
  framed: '',
  messages: { ui_read_failed: 'read: %s', ui_unread_failed: 'unread: %s', ui_http_error: '%s (%d)' },
} as unknown as Boot;

// A server whose read marks wait for the test to answer them, each as what answers says of its slug
function server(answers: Record<string, () => Promise<Response>>) {
  const sent: string[] = [];
  const fetch = (url: string, init?: RequestInit) => {
    const slug = url.split('/')[3] ?? '';
    sent.push(`${init?.method ?? 'GET'} ${slug}`);
    return answers[slug]?.() ?? Promise.resolve(new Response('{}'));
  };
  return { fetch, sent };
}

function made(reading: Promise<undefined> | null = null) {
  setBoot(BOOT);
  const notice = createNotice();
  const openPage = createOpenPage({ boot: () => BOOT, search: () => '' });
  const loads: boolean[] = [];
  // As the list's read does, the read going is let go once it ends
  let going = reading;
  void reading?.then(() => {
    going = null;
  });
  const pages = {
    whileReading: () => going,
    load: (polled = false) => {
      loads.push(polled);
      return Promise.resolve();
    },
    state: createStore({ entries: [{ slug: 'old', archived: true }], tasks: {}, readWhy: '' }),
  } as unknown as Pages;
  const reads = createReads({ client: createClient(), notice, pages, openPage });
  return { reads, notice, openPage, loads };
}

const READ = { read: { n: 2, at: 't' }, say: true };

describe(createReads, () => {
  it("sends one mark at a time, each page's last, the one before it dropped, and reads the list once all are sent", async () => {
    expect.assertions(3);
    const first = Promise.withResolvers<Response>();
    const s = server({ a: () => first.promise, b: () => Promise.resolve(new Response('{"cleared":true}')) });
    vi.stubGlobal('fetch', s.fetch);
    const { reads, loads } = made();
    const a = reads.tellRead('a', READ);
    const b1 = reads.tellRead('b', { read: { n: 1, at: 't' }, say: false });
    const b2 = reads.tellRead('b', READ);
    first.resolve(new Response('{}'));
    await expect(Promise.all([a, b1, b2])).resolves.toStrictEqual([
      { kind: 'sent', cleared: false },
      { kind: 'dropped' },
      { kind: 'sent', cleared: true },
    ]);
    expect(s.sent).toStrictEqual(['POST a', 'POST b']);
    expect(loads).toStrictEqual([true]);
  });

  it("waits for the list's read before it sends", async () => {
    expect.assertions(2);
    const s = server({});
    vi.stubGlobal('fetch', s.fetch);
    const reading = Promise.withResolvers<undefined>();
    const { reads } = made(reading.promise);
    const sent = reads.tellRead('a', READ);
    await Promise.resolve();
    expect(s.sent).toStrictEqual([]);
    reading.resolve(undefined);
    await sent;
    expect(s.sent).toStrictEqual(['POST a']);
  });

  it('says a read that replaces one to be said (S428), not an unread mark, and not twice once the first is out', async () => {
    expect.assertions(4);
    const s = server({ a: () => Promise.resolve(new Response('{"cleared":true}')) });
    vi.stubGlobal('fetch', s.fetch);
    const quiet = { read: { n: 3, at: 't2' }, say: false };
    // Opened, the page reads itself anew at once: the second read waits in the first's place, and is said
    const reading = Promise.withResolvers<undefined>();
    const first = made(reading.promise);
    const opened = first.reads.tellRead('a', READ);
    const again = first.reads.tellRead('a', quiet);
    reading.resolve(undefined);
    expect([await opened, await again]).toStrictEqual([{ kind: 'dropped' }, { kind: 'sent', cleared: true }]);
    // A mark unread in its place is its own
    const held = Promise.withResolvers<undefined>();
    const unread = made(held.promise);
    void unread.reads.tellRead('a', READ);
    const marked = unread.reads.tellRead('a', null);
    held.resolve(undefined);
    await expect(marked).resolves.toStrictEqual({ kind: 'sent', cleared: false });
    // The first sent already, the second is not said again
    const late = made();
    const sent = late.reads.tellRead('a', READ);
    await Promise.resolve();
    const after = late.reads.tellRead('a', quiet);
    await expect(sent).resolves.toStrictEqual({ kind: 'sent', cleared: true });
    await expect(after).resolves.toStrictEqual({ kind: 'sent', cleared: false });
  });

  it('says a refused mark on the error line, a read cut off not but an unread one, and passes a page gone', async () => {
    expect.assertions(4);
    const s = server({
      refused: () => Promise.resolve(new Response('locked', { status: 500 })),
      off: () => Promise.reject(new TypeError('down')),
      gone: () => Promise.resolve(new Response('', { status: 404 })),
    });
    vi.stubGlobal('fetch', s.fetch);
    const { reads, notice } = made();
    await expect(reads.tellRead('refused', READ)).resolves.toStrictEqual({
      kind: 'failed',
      reason: 'read: locked (500)',
      said: true,
    });
    await expect(reads.tellRead('off', READ)).resolves.toStrictEqual({ kind: 'failed', reason: 'down', said: false });
    await expect(reads.tellRead('gone', READ)).resolves.toStrictEqual({ kind: 'gone' });
    await reads.tellRead('off', null);
    expect(notice.said.get()).toStrictEqual({ message: 'unread: down', from: 'read' });
  });

  it('tells each mark the server took, read or unread, and none refused, cut off or of a page gone', async () => {
    expect.assertions(1);
    vi.stubGlobal(
      'fetch',
      server({
        refused: () => Promise.resolve(new Response('locked', { status: 500 })),
        off: () => Promise.reject(new TypeError('down')),
        gone: () => Promise.resolve(new Response('', { status: 404 })),
      }).fetch,
    );
    const { reads } = made();
    const told: (string | undefined)[] = [];
    reads.answered.subscribe(() => told.push(reads.answered.get()?.slug));
    for (const slug of ['a', 'refused', 'off', 'gone']) await reads.tellRead(slug, READ);
    await reads.tellRead('a', null);
    expect(told).toStrictEqual(['a', 'a']);
  });

  it('reads no answer of a read not to be said, which may hold no JSON', async () => {
    expect.assertions(1);
    vi.stubGlobal('fetch', server({ a: () => Promise.resolve(new Response('ok')) }).fetch);
    const { reads } = made();
    await expect(reads.tellRead('a', { read: { n: 1, at: 't' }, say: false })).resolves.toStrictEqual({
      kind: 'sent',
      cleared: false,
    });
  });

  it('keeps the page shown unread, lets it go when its mark fails, and marks no archived page', async () => {
    expect.assertions(3);
    vi.stubGlobal('fetch', server({ a: () => Promise.reject(new TypeError('down')) }).fetch);
    const { reads, openPage } = made();
    openPage.picked.set({ slug: 'a', file: '', v: 0 });
    const marked = reads.markUnread('a');
    expect(reads.keptUnread.get()).toBe('a');
    await marked;
    expect(reads.keptUnread.get()).toBeNull();
    expect(reads.markUnread('old')).toBeNull();
  });
});
