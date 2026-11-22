import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createClient } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import type { Pages, PagesState } from '@/stores/pages';
import { createTaskWrites } from '@/stores/tasks';
import type { TaskWritten } from '@/types/task';

vi.stubGlobal('window', globalThis);

const boot = (screen: Boot['screen']) =>
  ({
    screen,
    messages: { ui_task_no_rev: 'no rev', ui_task_stale: 'stale: %s', ui_http_error: '%s (%d)' },
  }) as unknown as Boot;

// The list's store as the writes reach it: what they call, in order
function made(screen: Boot['screen'], answer: Response) {
  setBoot(boot(screen));
  const called: string[] = [];
  const sent: string[] = [];
  vi.stubGlobal('fetch', (_url: string, init?: RequestInit) => {
    sent.push(typeof init?.body === 'string' ? init.body : '');
    return Promise.resolve(answer);
  });
  const pages = {
    state: createStore<PagesState>({
      read: true,
      entries: [],
      tasks: { error: '', tasks: {}, raw: '' },
      readWhy: '',
      drawWhy: '',
    }),
    load: () => {
      called.push('load');
      return Promise.resolve();
    },
    rereadTasks: () => {
      called.push('rereadTasks');
      return Promise.resolve();
    },
    takeTasks: (written: readonly TaskWritten[]) => {
      called.push(`takeTasks ${written.map((w) => w.task).join()}`);
    },
  } as unknown as Pages;
  return { writes: createTaskWrites({ client: createClient(), pages, boot: () => boot(screen) }), called, sent };
}

describe(createTaskWrites, () => {
  it('sends no write of a task said to be there with no rev', async () => {
    expect.assertions(2);
    const { writes, sent } = made('tasks', new Response('{}'));
    await expect(writes.writeTask('p', 'x', { new: false })).resolves.toMatchObject({ ok: false, text: 'no rev' });
    expect(sent).toStrictEqual([]);
  });

  it('reads the list again when refused as stale, a frame its tasks too', async () => {
    expect.assertions(2);
    const list = made('tasks', new Response('changed', { status: 409 }));
    await list.writes.writeTask('p', 'x', { new: false, rev: 1 });
    expect(list.called).toStrictEqual(['load']);
    const frame = made('frame', new Response('changed', { status: 409 }));
    await frame.writes.writeTask('p', 'x', { new: false, rev: 1 });
    expect(frame.called).toStrictEqual(['rereadTasks', 'load']);
  });

  it('puts in the tasks a write answers with, and tells the rev of the first', async () => {
    expect.assertions(2);
    const answer = { tasks: [{ taskProject: 'p', task: 'x', row: { state: 'todo', at: 't', rev: 4 } }], pages: [] };
    const { writes, called } = made('tasks', Response.json(answer));
    await expect(writes.writeTask('p', 'x', { new: true })).resolves.toMatchObject({ ok: true, rev: 4 });
    expect(called).toStrictEqual(['takeTasks x']);
  });

  it('puts a page on no category by sending the category empty, not leaving it out', async () => {
    expect.assertions(1);
    const { writes, sent } = made('tasks', Response.json({ tasks: [], pages: [] }));
    await writes.putPageOn({ slug: 'memo' }, '#12', { new: false, rev: 7 }, '');
    expect(sent).toStrictEqual(['{"slugs":["memo"],"task":"#12","category":"","new":false,"rev":7}']);
  });
});
