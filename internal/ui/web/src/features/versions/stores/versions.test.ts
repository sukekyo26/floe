import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createVersions } from '@/features/versions/stores/versions';
import type { VersionList } from '@/features/versions/utils/version-words';
import type { Client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { createOpenPage, type OpenAt } from '@/stores/open-page';
import type { Reads } from '@/stores/reads';
import type { Page } from '@/types/page';

const listOf = (...ns: number[]): VersionList => ({ versions: ns.map((n) => ({ n, at: 't' })), read: 0 });
const page = (slug = 'a', version = 3): Page => ({ slug, version, versionCount: 3 }) as Page;
const at = (slug = 'a', v = 0): OpenAt => ({ slug, file: '', v });
const settle = () => new Promise((go) => setTimeout(go, 0));

// made makes the versions over fakes: each read of /versions waits for the test to answer it (answer, in order), the
// screen is the list's or the frame of page a at ?v=version
function made({ frame = false, version = '', modal = false, move = { focus: false, kept: false } } = {}) {
  setBoot({ messages: new Proxy({}, { get: (_, key) => String(key) }) } as unknown as Boot);
  const boot = () => ({ screen: frame ? 'frame' : 'list', framed: frame ? 'a' : '', version }) as Boot;
  const asked: string[] = [];
  // Each read answered with a list, a refusal (a Response) or the server not reached (an Error)
  const answers: ((with_: VersionList | Response | Error) => void)[] = [];
  const client = {
    api: (url: string) => {
      asked.push(url);
      return new Promise<Response>((done, fail) => {
        answers.push((with_) => {
          if (with_ instanceof Error) fail(with_);
          else done(with_ instanceof Response ? with_ : new Response(JSON.stringify(with_)));
        });
      });
    },
  } as unknown as Client;
  const openPage = createOpenPage({ boot, search: () => '' });
  const reads = { keptUnread: createStore<string | null>(null), answered: createStore<{ slug: string } | null>(null) };
  const log: string[] = [];
  const closes: (() => void)[] = [];
  let modalOpen = modal;
  // The modal closes: what waited for it runs
  const close = () => {
    modalOpen = false;
    for (const run of closes.splice(0)) run();
  };
  const v = createVersions({
    client,
    openPage,
    reads: reads as unknown as Reads,
    versionMove: {
      keep: (focus, kept) => log.push(`keep ${String(focus)} ${String(kept)}`),
      take: () => move,
    },
    boot,
    go: (href) => log.push(`go ${href}`),
    reload: () => log.push('reload'),
    afterModal: (run) => {
      if (modalOpen) closes.push(run);
      return modalOpen;
    },
  });
  const answer = async (list: VersionList | Response | Error) => {
    answers.shift()?.(list);
    await settle();
  };
  // Opens the page of x beside the list, and wants its versions as the choice's effect does
  const open = (x: OpenAt, a: Page = page(x.slug)) => {
    openPage.picked.set(x);
    v.want(x, a);
  };
  return { v, asked, answer, open, openPage, reads, log, closes, close };
}

describe(createVersions, () => {
  it('reads the versions of an entry once, and again as its key changes', async () => {
    expect.assertions(2);
    const m = made();
    m.open(at());
    await m.answer(listOf(1, 2, 3));
    m.v.want(at(), page());
    m.v.want(at(), page('a', 4));
    expect(m.asked).toStrictEqual(['/api/pages/a/versions', '/api/pages/a/versions']);
    expect(m.v.view(at(), page()).select?.numbers).toBe('1,2,3');
  });

  it("drops an answer come after another was asked (TestUIVersionAnswers), and shows no other page's versions", async () => {
    expect.assertions(3);
    const m = made();
    m.open(at('d', 2));
    await m.answer(listOf(1, 2, 3));
    m.open(at('e'));
    // At once, before its answer: the choice is hidden, with no options
    expect(m.v.view(at('e'), page('e')).select).toBeNull();
    m.open(at('d', 2));
    await m.answer(listOf(1, 2, 3, 4));
    await m.answer(listOf(1, 2, 3));
    expect(m.v.state.get().list?.versions).toHaveLength(3);
    expect(m.openPage.picked.get()?.v).toBe(2);
  });

  it('reads nothing of a page whose versions are not wanted, and keeps the words of one gone but of no page', () => {
    expect.assertions(2);
    const m = made({ frame: true, version: 'gone' });
    m.v.arrive();
    m.v.want(at('a'), { slug: 'a', version: 1, versionCount: 1 } as Page);
    m.reads.answered.set({ slug: 'a' });
    expect([m.asked, m.v.state.get().gone]).toStrictEqual([[], true]);
    m.v.want(at('a'), null);
    expect(m.v.state.get().gone).toBe(false);
  });

  it('keeps the words of one gone and the focus carried over through the first read of the frame', async () => {
    expect.assertions(2);
    const m = made({ frame: true, version: 'gone', move: { focus: true, kept: true } });
    m.v.arrive();
    m.v.want(at(), page());
    await m.answer(listOf(1, 2, 3));
    expect([m.v.state.get().gone, m.v.state.get().focus, m.reads.keptUnread.get()]).toStrictEqual([true, true, 'a']);
    m.v.takeFocus(null);
    expect(m.v.state.get().focus).toBe(true);
  });

  it('reads again on a mark of the page wanted alone', async () => {
    expect.assertions(1);
    const m = made();
    m.open(at());
    await m.answer(listOf(1, 2, 3));
    m.reads.answered.set({ slug: 'b' });
    m.reads.answered.set({ slug: 'a' });
    m.reads.answered.set({ slug: 'a' });
    expect(m.asked).toHaveLength(3);
  });

  it('picks the latest as 0 even if the list of pages knows it not (TestUIVersionPickAhead), beside the list and whole', async () => {
    expect.assertions(2);
    const m = made();
    m.open(at(), page('a', 3));
    await m.answer(listOf(1, 2, 3, 4));
    m.v.pick(4, { focus: true, held: false });
    m.v.pick(2, { focus: false, held: false });
    expect([m.openPage.picked.get()?.v, m.v.state.get().focus]).toStrictEqual([2, false]);
    const w = made({ frame: true });
    w.reads.keptUnread.set('a');
    w.v.want(at(), page());
    await w.answer(listOf(1, 2, 3));
    w.v.pick(3, { focus: false, held: true });
    w.v.pick(1, { focus: false, held: false });
    expect(w.log).toStrictEqual(['keep true true', 'go /a/a/', 'keep false true', 'go /a/a/?v=1']);
  });

  it('leaves a version gone beside the list for the latest, kept unread, with its words, ended by a row picked', async () => {
    expect.assertions(2);
    const m = made();
    m.open(at('a', 2));
    await m.answer(listOf(1, 3));
    expect([m.openPage.picked.get()?.v, m.reads.keptUnread.get(), m.v.state.get().gone]).toStrictEqual([0, 'a', true]);
    m.open(at('b'));
    expect(m.v.state.get().gone).toBe(false);
  });

  it('leaves a version removed while shown, the list read before, with the words of one gone', async () => {
    expect.assertions(1);
    const m = made();
    m.open(at('a'));
    await m.answer(listOf(1, 2, 3));
    m.v.pick(2, { focus: false, held: true });
    m.v.want(at('a', 2), page());
    m.v.want(at('a', 2), { slug: 'a', version: 3, versionCount: 2 } as Page);
    await m.answer(listOf(1, 3));
    m.v.want(at('a'), { slug: 'a', version: 3, versionCount: 2 } as Page);
    expect([m.openPage.picked.get()?.v, m.v.state.get().gone]).toStrictEqual([0, true]);
  });

  it('writes nothing of an answer come once another row was picked, before the choice wanted it', async () => {
    expect.assertions(1);
    const m = made();
    m.open(at('a', 2));
    m.openPage.picked.set(at('b'));
    await m.answer(listOf(1, 3));
    expect([
      m.openPage.picked.get(),
      m.reads.keptUnread.get(),
      m.v.state.get().gone,
      m.v.state.get().list,
    ]).toStrictEqual([at('b'), null, false, null]);
  });

  it('leaves the frame for the latest at once, with no list drawn, or once the modal open closes, waiting once', async () => {
    expect.assertions(4);
    const now = made({ frame: true, version: '2' });
    now.v.want(at('a', 2), page());
    await now.answer(listOf(1, 3));
    expect([now.log, now.v.state.get().list]).toStrictEqual([['keep false true', 'reload'], null]);
    const m = made({ frame: true, version: '2', modal: true });
    m.v.want(at('a', 2), page());
    await m.answer(listOf(1, 2, 3));
    m.v.want(at('a', 2), page('a', 4));
    await m.answer(listOf(1, 3));
    // The versions read again on a mark, still the modal open: the choice and words of before stay, one wait
    m.reads.answered.set({ slug: 'a' });
    await m.answer(listOf(1, 3));
    m.v.want(at('a', 2), page('a', 5));
    expect([m.closes.length, m.v.state.get().list?.versions.length, m.asked.length, m.log]).toStrictEqual([
      1,
      3,
      3,
      [],
    ]);
    m.close();
    expect(m.v.state.get().waiting).toBe(false);
    await m.answer(listOf(1, 3));
    expect(m.log).toStrictEqual(['keep false true', 'reload']);
  });

  it('changes nothing on a read refused or not reaching the server', async () => {
    expect.assertions(1);
    const m = made();
    m.open(at('a', 2));
    await m.answer(listOf(1, 2, 3));
    const before = m.v.state.get();
    m.v.want(at('a', 2), page('a', 4));
    await m.answer(new Response('locked', { status: 500 }));
    m.v.want(at('a', 2), page('a', 5));
    await m.answer(new TypeError('down'));
    expect([m.v.state.get(), m.openPage.picked.get()?.v]).toStrictEqual([before, 2]);
  });

  it('writes nothing of an answer to again come once another row was picked', async () => {
    expect.assertions(1);
    const m = made();
    m.open(at('a', 2));
    await m.answer(listOf(1, 2, 3));
    m.reads.answered.set({ slug: 'a' });
    m.openPage.picked.set(at('b'));
    await m.answer(listOf(1, 3));
    expect([m.openPage.picked.get(), m.reads.keptUnread.get(), m.v.state.get().gone]).toStrictEqual([
      at('b'),
      null,
      false,
    ]);
  });

  it('leaves for the latest on the list read before, the key the same, reading nothing', async () => {
    expect.assertions(1);
    const m = made();
    m.open(at());
    await m.answer(listOf(1, 2, 3));
    m.open(at('a', 3));
    expect([m.openPage.picked.get()?.v, m.asked.length]).toStrictEqual([0, 1]);
  });

  it('drops an answer come after a want that reads nothing, in the frame too, which checks no row', async () => {
    expect.assertions(1);
    const m = made({ frame: true });
    m.v.want(at(), page());
    m.v.want(at(), { slug: 'a', version: 1, versionCount: 1 } as Page);
    await m.answer(listOf(1, 2, 3));
    expect(m.v.state.get().list).toBeNull();
  });

  it('gives the choice the focus asked for once it shows, and takes it all the same from one hidden', () => {
    expect.assertions(2);
    const m = made({ frame: true, move: { focus: true, kept: false } });
    m.v.arrive();
    const shown = { hidden: false, old: false, value: 3, numbers: '1,2,3', options: [] };
    expect([m.v.takeFocus(null), m.v.takeFocus({ ...shown, hidden: true }), m.v.takeFocus(shown)]).toStrictEqual([
      false,
      false,
      false,
    ]);
    m.v.arrive();
    expect(m.v.takeFocus(shown)).toBe(true);
  });

  it('keeps the words of a version gone in the frame, whatever is picked there', () => {
    expect.assertions(1);
    const m = made({ frame: true, version: 'gone' });
    m.v.arrive();
    m.openPage.picked.set(at('b'));
    expect(m.v.state.get().gone).toBe(true);
  });
});
