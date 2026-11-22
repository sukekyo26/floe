import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createPageReads } from '@/features/reader/stores/page-reads';
import { createStore } from '@/lib/store';
import type { OpenPage } from '@/stores/open-page';
import type { PageActions } from '@/stores/page-actions';
import type { PageNote } from '@/stores/page-note';
import type { Pages } from '@/stores/pages';
import type { ReadOutcome, Reads, Told } from '@/stores/reads';
import { createToasts } from '@/stores/toasts';

const MARK = { n: 3, at: 't' };
const none = () => undefined;

// made makes the reads over fakes: each read waits for the test to answer it (answer), the page shown is shown.slug
function made({ archived = false } = {}) {
  setBoot({ messages: new Proxy({}, { get: (_, key) => String(key) }) } as unknown as Boot);
  const log: string[] = [];
  const keptUnread = createStore<string | null>(null);
  const answers: ((outcome: ReadOutcome) => void)[] = [];
  const reads = {
    keptUnread,
    tellRead: (slug: string, told: Told) => {
      log.push(`tell ${slug} say=${String(told?.say)}`);
      return new Promise<ReadOutcome>((done) => answers.push(done));
    },
  } as unknown as Reads;
  const toasts = createToasts();
  let to: number | null = null;
  const note = {
    to: () => to,
    clear: () => {
      to = null;
      log.push('note cleared');
    },
    show: (slug: string, n: number) => {
      to = n;
      log.push(`note ${slug} ${String(n)}`);
      return Promise.resolve();
    },
  } as unknown as PageNote;
  const shown = { slug: 'a' };
  const pr = createPageReads({
    reads,
    toasts,
    note,
    pages: { state: createStore({ entries: [{ slug: 'a', title: 'A', archived }] }) } as unknown as Pages,
    openPage: { shownSlug: () => shown.slug } as unknown as OpenPage,
    actions: { markUnreadSaid: () => Promise.resolve() } as unknown as PageActions,
  });
  const answer = (outcome: ReadOutcome) => answers.shift()?.(outcome);
  return { pr, log, toasts, keptUnread, shown, answer };
}
const texts = (t: ReturnType<typeof createToasts>) => t.shown.get().map((x) => x.text);
const settle = () => new Promise((go) => setTimeout(go, 0));

describe(createPageReads, () => {
  it('takes the toast of the page shown before away as another opens, and no other', () => {
    expect.assertions(1);
    const { pr, toasts } = made();
    toasts.toast('read', { page: true });
    toasts.toast('saved');
    pr.opened('b');
    expect(texts(toasts)).toStrictEqual(['saved']);
  });

  it('says the first read of a page opened, with a way back to unread and the words of its versions', async () => {
    expect.assertions(3);
    const { pr, log, toasts, answer } = made();
    const first = pr.served('a', MARK, none);
    answer({ kind: 'sent', cleared: true });
    await first;
    expect(texts(toasts)).toStrictEqual(['ui_read_done']);
    expect(toasts.shown.get()[0]?.button).toMatchObject({ undo: false, label: 'ui_mark_unread' });
    // Again, the page drew itself anew: told, not to be said
    const again = pr.served('a', MARK, none);
    answer({ kind: 'sent', cleared: false });
    await again;
    expect(log).toStrictEqual(['tell a say=true', 'note a 3', 'tell a say=false']);
  });

  it('says nothing marked unread before the answer came (TestUIUnreadKept 220), nor of a page left meanwhile', async () => {
    expect.assertions(2);
    const kept = made();
    const read = kept.pr.served('a', MARK, none);
    kept.keptUnread.set('a');
    kept.answer({ kind: 'sent', cleared: true });
    await read;
    expect([texts(kept.toasts), kept.log.filter((x) => x.startsWith('note'))]).toStrictEqual([[], []]);
    const left = made();
    const p = left.pr.served('a', MARK, none);
    left.shown.slug = 'b';
    left.answer({ kind: 'sent', cleared: true });
    await p;
    expect([texts(left.toasts), left.log]).toStrictEqual([[], ['tell a say=true']]);
  });

  it('says the words of an archived page alone, and sends nothing of a page kept unread', async () => {
    expect.assertions(2);
    const old = made({ archived: true });
    const p = old.pr.served('a', MARK, none);
    old.answer({ kind: 'sent', cleared: true });
    await p;
    expect([texts(old.toasts), old.log]).toStrictEqual([[], ['tell a say=true', 'note a 3']]);
    const kept = made();
    kept.keptUnread.set('a');
    await kept.pr.served('a', MARK, none);
    expect(kept.log).toStrictEqual([]);
  });

  it('keeps a page unread through its versions until another is opened, and clears the words of another version', async () => {
    expect.assertions(3);
    const { pr, keptUnread, log } = made();
    pr.opened('a');
    keptUnread.set('a');
    pr.opened('a');
    expect(keptUnread.get()).toBe('a');
    pr.opened('b');
    expect(keptUnread.get()).toBeNull();
    // Words of version 3, then the page drawn at version 4: they go
    const words = made();
    const p = words.pr.served('a', MARK, none);
    words.answer({ kind: 'sent', cleared: true });
    await p;
    void words.pr.served('a', { n: 4, at: 'u' }, none);
    await settle();
    expect([log.at(-1), words.log.includes('note cleared')]).toStrictEqual(['note cleared', true]);
  });

  it('says a read whose answer tells it cleared though it came second (S428: the read store kept it to be said)', async () => {
    expect.assertions(1);
    const { pr, toasts, answer } = made();
    void pr.served('a', MARK, none);
    const second = pr.served('a', MARK, none);
    answer({ kind: 'dropped' });
    answer({ kind: 'sent', cleared: true });
    await second;
    expect(texts(toasts)).toStrictEqual(['ui_read_done']);
  });
});
