import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createDiff } from '@/features/versions/stores/diff';
import type { DiffAnswer, Section } from '@/features/versions/utils/diff-words';
import type { Client } from '@/lib/api-client';
import { createNotice } from '@/stores/notice';
import type { Page } from '@/types/page';

const settle = () => new Promise((go) => setTimeout(go, 0));
const page = (versionCount = 3): Page => ({ slug: 'a', versionCount }) as Page;
const LIST = { versions: [1, 2, 3].map((n) => ({ n, at: 't' })), read: 0 };
const section = (place?: number): Section => ({
  state: 'changed',
  level: 1,
  heading: [],
  ...(place === undefined ? {} : { place }),
});
// An answer for the pair, its places and sections given
const answerOf = (
  from: number,
  to: number,
  places: number[] = [0, 1],
  sections: Section[] = [section(0), section(1)],
): DiffAnswer => ({
  from: { n: from },
  to: { n: to },
  key: 'k',
  counts: { changed: 1, added: 0, removed: 0, same: 0 },
  sections,
  places,
});

// made makes the diff over a fake server: each read waits for the test to answer it, in order, with a value, a refusal
// (a Response) or the server not reached (an Error); a read called off fails as fetch does
function made(search = '?diff') {
  setBoot({ messages: new Proxy({}, { get: (_, key) => `${String(key)} %s` }) } as unknown as Boot);
  const asked: { url: string; signal: AbortSignal }[] = [];
  const answers: { signal: AbortSignal; go: (with_: unknown) => void }[] = [];
  const client = {
    api: (url: string, init?: RequestInit) => {
      const signal = init?.signal ?? new AbortController().signal;
      asked.push({ url, signal });
      return new Promise<Response>((done, fail) => {
        signal.addEventListener('abort', () => {
          fail(new DOMException('called off', 'AbortError'));
        });
        answers.push({
          signal,
          go: (with_) => {
            if (with_ instanceof Error) fail(with_);
            else done(with_ instanceof Response ? with_ : new Response(JSON.stringify(with_)));
          },
        });
      });
    },
  } as unknown as Client;
  const notice = createNotice();
  // The URL's query follows what is written, as the browser's does
  const urls: string[] = [];
  let query = search;
  const diff = createDiff({
    client,
    notice,
    search: () => query,
    replace: (href) => {
      urls.push(href);
      query = new URL(href, 'http://floe').search;
    },
  });
  // Answers the first read still going: one called off has failed already
  const answer = async (with_: unknown) => {
    const i = answers.findIndex((x) => !x.signal.aborted);
    answers
      .splice(0, i + 1)
      .at(-1)
      ?.go(with_);
    await settle();
  };
  return { diff, asked, answer, notice, urls };
}

describe(createDiff, () => {
  it('opens once, the second call reading nothing, and shows the first pair with no frame to load before its answer', async () => {
    expect.assertions(3);
    const m = made();
    m.diff.open(page());
    m.diff.open(page());
    await m.answer(LIST);
    expect([m.asked.map((x) => x.url), m.diff.state.get().turn]).toStrictEqual([
      ['/api/pages/a/versions', '/api/pages/a/diff?from=2&to=3'],
      0,
    ]);
    await m.answer(answerOf(2, 3));
    expect([m.diff.state.get().turn, m.diff.state.get().at, m.diff.state.get().pair]).toStrictEqual([
      1,
      -1,
      { from: 2, to: 3 },
    ]);
    expect(m.urls).toStrictEqual([]);
  });

  it('has none to compare of a page of one version, and says a list of versions not read', async () => {
    expect.assertions(2);
    const one = made();
    one.diff.open(page(1));
    expect([one.diff.state.get().none, one.asked]).toStrictEqual([true, []]);
    const off = made();
    off.diff.open(page());
    await off.answer(new Response('locked', { status: 500 }));
    expect([off.notice.said.get().message, off.diff.state.get().turn]).toStrictEqual(['ui_diff_failed locked', 0]);
  });

  it('writes the URL of a pair that cannot be compared, and keeps saying so through a read again, until one is chosen', async () => {
    expect.assertions(3);
    const m = made('?diff&from=3&to=1');
    m.diff.open(page());
    await m.answer(LIST);
    expect([m.urls, m.diff.state.get().bad]).toStrictEqual([['/a/a/?diff'], true]);
    m.diff.reopen(page());
    await m.answer(LIST);
    expect(m.diff.state.get().bad).toBe(true);
    m.diff.choose('from', 1);
    expect([m.diff.state.get().bad, m.urls.at(-1)]).toStrictEqual([false, '/a/a/?diff&from=1&to=3']);
  });

  it('calls the read before off as another is asked, the versions too, and an answer called off does nothing', async () => {
    expect.assertions(3);
    const m = made();
    m.diff.open(page());
    m.diff.reopen(page());
    await settle();
    expect([m.asked[0]?.signal.aborted, m.notice.said.get().message]).toStrictEqual([true, '']);
    await m.answer(LIST);
    m.diff.choose('from', 1);
    expect(m.asked.filter((x) => x.url.includes('/diff')).map((x) => x.signal.aborted)).toStrictEqual([true, false]);
    await m.answer(answerOf(1, 3));
    expect([m.diff.state.get().answer?.from.n, m.notice.said.get().message]).toStrictEqual([1, '']);
  });

  it("calls a diff being read off as the page's versions changed, and reads them again", async () => {
    expect.assertions(2);
    const m = made();
    m.diff.open(page());
    await m.answer(LIST);
    m.diff.reopen(page());
    expect(m.asked.map((x) => [x.url.split('/').at(-1), x.signal.aborted])).toStrictEqual([
      ['versions', true],
      ['diff?from=2&to=3', true],
      ['versions', false],
    ]);
    await m.answer(LIST);
    await m.answer(answerOf(2, 3));
    expect(m.diff.state.get().turn).toBe(1);
  });

  it('goes back to the pair shown when a read fails, saying so, and takes the words back as one comes', async () => {
    expect.assertions(3);
    const m = made();
    m.diff.open(page());
    await m.answer(LIST);
    await m.answer(answerOf(2, 3));
    m.diff.choose('from', 1);
    await m.answer(new Response('gone', { status: 500 }));
    expect([m.diff.state.get().pair, m.urls.at(-1), m.notice.said.get().message]).toStrictEqual([
      { from: 2, to: 3 },
      '/a/a/?diff&from=2&to=3',
      'ui_diff_failed gone',
    ]);
    m.diff.choose('from', 1);
    await m.answer(answerOf(1, 3));
    expect(m.notice.said.get().message).toBe('');
    // With no answer shown, a failure goes back to nothing
    const first = made();
    first.diff.open(page());
    await first.answer(LIST);
    await first.answer(new TypeError('down'));
    expect([first.diff.state.get().pair, first.urls]).toStrictEqual([{ from: 2, to: 3 }, []]);
  });

  it('steps through the places within them, the section of each as the answer tells, the frame loaded each time', async () => {
    expect.assertions(3);
    const m = made();
    m.diff.open(page());
    await m.answer(LIST);
    // Two sections share the place 0: the section of a press is the one pressed
    await m.answer(answerOf(2, 3, [0, 2], [section(0), section(0), section(1)]));
    m.diff.step(-1);
    m.diff.step(1);
    m.diff.step(1);
    m.diff.step(1);
    const { at, section: shown, turn } = m.diff.state.get();
    expect([at, shown, turn]).toStrictEqual([1, 2, 3]);
    m.diff.place(1);
    m.diff.place(1);
    expect([m.diff.state.get().at, m.diff.state.get().section, m.diff.state.get().turn]).toStrictEqual([0, 1, 5]);
    m.diff.place(9);
    expect(m.diff.state.get().turn).toBe(5);
  });
});
