import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createClient } from '@/lib/api-client';
import type { Kept } from '@/lib/storage';
import { createNotice } from '@/stores/notice';
import { createFolders } from '@/stores/open-folders';
import { createPages } from '@/stores/pages';

vi.stubGlobal('window', globalThis);

const MESSAGES = {
  ui_load_failed: 'failed: %s',
  ui_load_slow: 'slow: %s %s',
  ui_tasks_failed: 'tasks failed: %s',
  ui_http_error: '%s (%d)',
} as unknown as Boot['messages'];
const boot = (screen: Boot['screen'] = 'list'): Boot => ({
  screen,
  lang: 'en',
  messages: MESSAGES,
  framed: screen === 'frame' ? 'f' : '',
  served: false,
  version: '',
  states: [],
  maxURLs: 10,
});

const memory = (): Kept => {
  const m = new Map<string, string>();
  return {
    read: (key) => m.get(key) ?? null,
    write: (key, value) => m.set(key, value),
    remove: (key) => m.delete(key),
  };
};

// A server of the list and the tasks: each request waits for the test to answer it, or is answered at once when auto
function server(auto = true) {
  const asked: { url: string; method: string; done: boolean; answer: (body: string, status?: number) => void }[] = [];
  let going = 0;
  let most = 0;
  let listsGoing = 0;
  let mostLists = 0;
  const state = { list: '[]', tasks: '{}', listStatus: 200, tasksStatus: 200 };
  const reply = (a: { url: string; method: string; answer: (body: string, status?: number) => void }) => {
    if (a.method !== 'GET') a.answer('{}');
    else if (a.url === '/api/pages') a.answer(state.list, state.listStatus);
    else if (a.url === '/api/tasks') a.answer(state.tasks, state.tasksStatus);
    else a.answer('{"dirs":[],"files":[],"more":0}');
  };
  const fetch = (url: string, init?: RequestInit) => {
    going++;
    most = Math.max(most, going);
    const list = url === '/api/pages';
    if (list) mostLists = Math.max(mostLists, ++listsGoing);
    const answer = Promise.withResolvers<Response>();
    const ask = {
      url,
      method: init?.method ?? 'GET',
      done: false,
      answer: (body: string, status = 200) => {
        if (ask.done) return;
        ask.done = true;
        going--;
        if (list) listsGoing--;
        answer.resolve(new Response(body, { status }));
      },
    };
    asked.push(ask);
    // As a real fetch does, a request called off or cut off rejects with its signal's reason
    init?.signal?.addEventListener('abort', () => {
      ask.done = true;
      going--;
      if (list) listsGoing--;
      answer.reject(init.signal?.reason);
    });
    if (auto && ask.method === 'GET') reply(ask);
    return answer.promise;
  };
  // flush answers every request waiting as the state says, a write with {}
  const flush = () => {
    for (const a of asked) reply(a);
  };
  return { fetch, asked, state, flush, most: () => most, mostLists: () => mostLists };
}

const tick = () =>
  new Promise((go) => {
    setTimeout(go, 1);
  });

// settle answers every request of s until p is done
async function settle(s: { flush: () => void }, p: Promise<unknown>) {
  const settled = { done: false };
  void p.finally(() => {
    settled.done = true;
  });
  for (let i = 0; i < 200 && !settled.done; i++) {
    s.flush();
    await tick();
  }
  await p;
}

// until waits for cond
async function until(cond: () => boolean) {
  for (let i = 0; i < 200 && !cond(); i++) await tick();
}

// A page's document as watch reads it
function page() {
  const handlers = new Set<() => void>();
  return {
    doc: {
      hidden: false,
      addEventListener: (_: string, h: () => void) => handlers.add(h),
      removeEventListener: (_: string, h: () => void) => handlers.delete(h),
    },
    show: () => {
      for (const h of handlers) h();
    },
  };
}

function made(screen: Boot['screen'] = 'list', options: { search?: string; readMs?: number; pressMs?: number } = {}) {
  // t() reads the messages of the boot values
  setBoot(boot(screen));
  const client = createClient();
  const notice = createNotice();
  const counts: [number, number][] = [];
  const shown = page();
  const search = () => options.search ?? '';
  const pages = createPages({
    client,
    notice,
    folders: createFolders({ client, notice, storage: memory(), search }),
    boot: () => boot(screen),
    search,
    shown: () => shown.doc as unknown as Document,
    count: (begun, ended) => counts.push([begun, ended]),
    readMs: options.readMs ?? 10000,
    pressMs: options.pressMs ?? 2000,
  });
  return { pages, notice, counts, shown, client };
}

describe(createPages, () => {
  it('joins the asks while a read waits, and never reads two at once', async () => {
    expect.assertions(2);
    const s = server(false);
    vi.stubGlobal('fetch', s.fetch);
    const { pages } = made();
    const lists = () => s.asked.filter((a) => a.url === '/api/pages').length;
    const first = pages.load();
    await until(() => lists() === 1);
    // Asked while the first reads: they join one read, which waits for the first
    const joined = Promise.all([pages.load(), pages.load(), pages.load()]);
    await tick();
    expect(lists()).toBe(1);
    await settle(s, Promise.all([first, joined]));
    expect({ lists: lists(), atOnce: s.mostLists() }).toStrictEqual({ lists: 2, atOnce: 1 });
  });

  it('counts each read begun and ended, a failed one and one joined too, the ended never past the begun', async () => {
    expect.assertions(2);
    const s = server();
    vi.stubGlobal('fetch', s.fetch);
    const { pages, counts } = made();
    await pages.load();
    await Promise.all([pages.load(), pages.load(), pages.load()]);
    s.state.listStatus = 500;
    await pages.load();
    expect(counts).toStrictEqual([
      [1, 0],
      [1, 1],
      [2, 1],
      [2, 2],
      [3, 2],
      [3, 3],
    ]);
    expect(counts.every(([begun, ended]) => ended <= begun)).toBe(true);
  });

  it('reads once no write waits, and again when one was sent while it read', async () => {
    expect.assertions(2);
    const s = server(false);
    vi.stubGlobal('fetch', s.fetch);
    const { pages, client } = made();
    const lists = () => s.asked.filter((a) => a.url === '/api/pages').length;
    const first = client.api('/api/pages/a', { method: 'PATCH' });
    const loaded = pages.load();
    await tick();
    expect(lists()).toBe(0);
    s.flush();
    await first;
    await until(() => lists() === 1);
    // A write sent while the list is read: the list is read again after it
    const second = client.api('/api/pages/b', { method: 'PATCH' });
    await settle(s, Promise.all([second, loaded]));
    expect(lists()).toBe(2);
  });

  it('says a read cut off by its time limit slow, and one refused failed', async () => {
    expect.assertions(2);
    vi.stubGlobal('fetch', server(false).fetch);
    const slow = made('list', { readMs: 20 });
    await slow.pages.load();
    expect(slow.notice.said.get()).toStrictEqual({ message: 'slow: 0.02 3', from: 'load' });
    const s = server();
    s.state.listStatus = 500;
    vi.stubGlobal('fetch', s.fetch);
    const refused = made();
    await refused.pages.load();
    expect(refused.notice.said.get()).toStrictEqual({ message: 'failed: HTTP 500', from: 'load' });
  });

  it('takes the list when the tasks fail, saying their mistake on the task screens alone, and reads no tasks in a frame', async () => {
    expect.assertions(4);
    const s = server();
    s.state.list = '[{"slug":"a","description":"","createdAt":"","updatedAt":"","taskProject":""}]';
    s.state.tasks = 'locked';
    s.state.tasksStatus = 500;
    vi.stubGlobal('fetch', s.fetch);
    const tasks = made('tasks');
    await tasks.pages.load();
    expect(tasks.pages.state.get().entries.map((a) => a.slug)).toStrictEqual(['a']);
    expect(tasks.notice.said.get().message).toBe('tasks failed: locked');
    vi.stubGlobal('fetch', () => Promise.reject(new TypeError('gone')));
    const thrown = made('tasks');
    await thrown.pages.load();
    expect(thrown.notice.said.get().message).toBe('failed: gone');
    const framed = server();
    vi.stubGlobal('fetch', framed.fetch);
    await made('frame').pages.load();
    expect(framed.asked.map((a) => a.url)).toStrictEqual(['/api/pages']);
  });

  it('takes no poll read meanwhile a press, but does once released and always an action read', async () => {
    expect.assertions(3);
    const s = server();
    vi.stubGlobal('fetch', s.fetch);
    const { pages } = made();
    pages.pressed();
    s.state.list = '[{"slug":"a","description":"","createdAt":"","updatedAt":"","taskProject":""}]';
    await pages.load(true);
    // A read held back takes no list: the list says nothing of having none
    expect([pages.state.get().entries.length, pages.state.get().read]).toStrictEqual([0, false]);
    await pages.load();
    expect([pages.state.get().entries.length, pages.state.get().read]).toStrictEqual([1, true]);
    s.state.list = '[]';
    pages.released();
    await pages.load(true);
    expect(pages.state.get().entries).toHaveLength(0);
  });

  it('takes a poll joined by an action as the action, while a press holds the polls', async () => {
    expect.assertions(1);
    const s = server(false);
    vi.stubGlobal('fetch', s.fetch);
    const { pages } = made();
    const first = pages.load(true);
    pages.pressed();
    const joined = Promise.all([pages.load(true), pages.load()]);
    s.state.list = '[{"slug":"b","description":"","createdAt":"","updatedAt":"","taskProject":""}]';
    await settle(s, Promise.all([first, joined]));
    expect(pages.state.get().entries).toHaveLength(1);
  });

  it('tells no one of an answer the same as the one taken', async () => {
    expect.assertions(1);
    vi.stubGlobal('fetch', server().fetch);
    const { pages } = made();
    await pages.load();
    let told = 0;
    pages.state.subscribe(() => told++);
    await pages.load();
    await pages.load(true);
    expect(told).toBe(0);
  });

  it('reads the folders open on the list and a task page, none on the task list, and the tree in a frame of a file', async () => {
    expect.assertions(4);
    const urls = async (screen: Boot['screen'], search: string) => {
      const s = server();
      s.state.list = '[{"slug":"d","folder":true,"description":"","createdAt":"","updatedAt":"","taskProject":""}]';
      vi.stubGlobal('fetch', s.fetch);
      const { pages } = made(screen, { search });
      await pages.load();
      return s.asked.filter((a) => a.url.includes('/files')).length;
    };
    await expect(urls('list', '?open=d')).resolves.toBe(1);
    await expect(urls('tasks', '?task=x&open=d')).resolves.toBe(1);
    await expect(urls('tasks', '?open=d')).resolves.toBe(0);
    await expect(urls('frame', '?file=a/b.html')).resolves.toBe(2);
  });

  it('polls every POLL_MS and as the tab shows again, not while hidden nor once stopped', async () => {
    expect.assertions(3);
    vi.useFakeTimers();
    vi.stubGlobal('fetch', server().fetch);
    const { pages, counts, shown } = made();
    const stop = pages.watch();
    await vi.advanceTimersByTimeAsync(3000);
    expect(counts.at(-1)).toStrictEqual([1, 1]);
    shown.show();
    await vi.advanceTimersByTimeAsync(0);
    shown.doc.hidden = true;
    await vi.advanceTimersByTimeAsync(3000);
    expect(counts.at(-1)).toStrictEqual([2, 2]);
    shown.doc.hidden = false;
    stop();
    await vi.advanceTimersByTimeAsync(6000);
    shown.show();
    expect(counts.at(-1)).toStrictEqual([2, 2]);
    vi.useRealTimers();
  });

  it('takes no poll answered within PRESS_MS of a press, and meanwhile clears its read mistake alone', async () => {
    expect.assertions(3);
    const s = server();
    vi.stubGlobal('fetch', s.fetch);
    const { pages, notice } = made();
    s.state.listStatus = 500;
    await pages.load();
    expect(notice.said.get()).toStrictEqual({ message: 'failed: HTTP 500', from: 'load' });
    s.state.listStatus = 200;
    s.state.list = '[{"slug":"a","description":"","createdAt":"","updatedAt":"","taskProject":""}]';
    pages.pressed();
    await pages.load(true);
    expect(notice.said.get().message).toBe('');
    expect(pages.state.get().entries).toHaveLength(0);
  });

  it('takes a poll answered once a press is past PRESS_MS', async () => {
    expect.assertions(1);
    const s = server();
    s.state.list = '[{"slug":"a","description":"","createdAt":"","updatedAt":"","taskProject":""}]';
    vi.stubGlobal('fetch', s.fetch);
    const { pages } = made('list', { pressMs: 30 });
    pages.pressed();
    // Late as the machine may be, the wait is only ever longer than PRESS_MS
    await new Promise((go) => {
      setTimeout(go, 40);
    });
    await pages.load(true);
    expect(pages.state.get().entries).toHaveLength(1);
  });

  it('knows a rev as read, as a write answered, and as read again alone in a frame', async () => {
    expect.assertions(3);
    const s = server();
    s.state.tasks = '{"p":{"x":{"state":"todo","at":"t","rev":3}}}';
    vi.stubGlobal('fetch', s.fetch);
    const { pages } = made('tasks');
    await pages.load();
    expect(pages.readRev('p', 'x')).toBe(3);
    pages.takeTasks([{ taskProject: 'p', task: 'x', row: { state: 'doing', at: 'u', rev: 5 } }]);
    expect(pages.readRev('p', 'x')).toBe(5);
    s.state.tasks = '{"p":{"x":{"state":"done","at":"v","rev":7}}}';
    await pages.rereadTasks();
    expect(pages.readRev('p', 'x')).toBe(7);
  });

  it('says a draw that failed after a read that failed and before the tasks, takes it back alone, and tells no one of the same again', async () => {
    expect.assertions(5);
    const s = server();
    s.state.tasksStatus = 500;
    vi.stubGlobal('fetch', s.fetch);
    const { pages, notice } = made('tasks');
    await settle(s, pages.load());
    pages.drawFailed('draw failed');
    expect(notice.said.get().message).toBe('draw failed');
    let told = 0;
    pages.state.subscribe(() => told++);
    pages.drawFailed('draw failed');
    expect(told).toBe(0);
    s.state.listStatus = 500;
    await settle(s, pages.load());
    expect(notice.said.get().message).toMatch(/^failed: /);
    s.state.listStatus = 200;
    await settle(s, pages.load());
    pages.drawFailed('');
    expect(notice.said.get().message).toMatch(/^tasks failed: /);
    expect(pages.ended.get()).toBe(3);
  });
});
