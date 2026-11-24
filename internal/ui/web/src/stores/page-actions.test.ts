import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createStore } from '@/lib/store';
import type { TasksAnswer } from '@/lib/tasks';
import { createNotice } from '@/stores/notice';
import type { OpenPage } from '@/stores/open-page';
import type { Order } from '@/stores/order';
import type { PageNote } from '@/stores/page-note';
import { createPageActions } from '@/stores/page-actions';
import type { PageWrites } from '@/stores/page-writes';
import type { Pages } from '@/stores/pages';
import { createPressOnce } from '@/stores/press-once';
import type { ReadOutcome, Reads } from '@/stores/reads';
import type { TaskWrites } from '@/stores/tasks';
import { buttonOf, createToasts } from '@/stores/toasts';
import { createUndoToast } from '@/stores/undo-toast';
import type { Page } from '@/types/page';

const page: Page = {
  slug: 'a',
  title: 'A',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'floe',
  task: { name: '#1', category: 'designs' },
};

interface Options {
  refuse?: string;
  framed?: string;
  shown?: string;
  outcome?: ReadOutcome | null;
  known?: boolean;
  stale?: boolean;
  screen?: string;
  // The reads of the list never end
  slow?: boolean;
}

// made makes the actions over fakes that log, in order, what they were asked
function made({
  refuse = '',
  framed = '',
  shown = '',
  outcome = { kind: 'sent', cleared: false },
  known = true,
  stale = false,
  screen = 'list',
  slow = false,
}: Options = {}) {
  const messages = new Proxy({} as Boot['messages'], { get: (_, key) => `${String(key)}:%s` });
  const boot = { screen, framed, messages } as unknown as Boot;
  setBoot(boot);
  const log: string[] = [];
  const now = { refuse };
  const toasts = createToasts();
  const notice = createNotice();
  const pages = {
    load: () => {
      log.push(`load after ${String(toasts.shown.get().length)} toasts`);
      return slow ? new Promise<void>(() => undefined) : Promise.resolve();
    },
    rereadTasks: () => {
      log.push('reread tasks');
      return Promise.resolve();
    },
    readRev: () => 7,
    state: createStore({ tasks: { tasks: { floe: known ? { '#1': { rev: 7 } } : {} } } }),
  } as unknown as Pages;
  const answer = stale
    ? { ok: false, status: 409, text: 'refused', stale: 'stale', rev: null, pages: [], written: [] }
    : { ok: true, status: 200, text: '', stale: '', rev: 8, pages: [], written: [] };
  const writes = {
    patchPage: (slug: string, body: unknown) => {
      log.push(`patch ${slug} ${JSON.stringify(body)}`);
      return Promise.resolve(now.refuse);
    },
    deletePage: (slug: string) => {
      log.push(`delete ${slug}`);
      return Promise.resolve(now.refuse);
    },
  } as unknown as PageWrites;
  const tasks = {
    changePageTask: (slug: string, body: unknown) => {
      log.push(`task ${slug} ${JSON.stringify(body)}`);
      return Promise.resolve(now.refuse);
    },
    putPageOn: (_a: Page, task: string, body: unknown, category: string): Promise<TasksAnswer> => {
      log.push(`put ${task} ${category} ${JSON.stringify(body)}`);
      return Promise.resolve(answer);
    },
  } as unknown as TaskWrites;
  const reads = { markUnread: () => (outcome ? Promise.resolve(outcome) : null) } as unknown as Reads;
  const actions = createPageActions({
    writes,
    tasks,
    pages,
    reads,
    notice,
    toasts,
    undoToast: createUndoToast({ toasts, notice, pages }),
    order: { keptBack: () => '/back' } as unknown as Order,
    openPage: { shownSlug: () => shown } as unknown as OpenPage,
    note: { clear: () => log.push('note cleared') } as unknown as PageNote,
    boot: () => boot,
    press: createPressOnce(),
    go: (href) => log.push(`go ${href}`),
    write: (text) => (text ? Promise.resolve() : Promise.reject(new Error('denied'))),
  });
  return { actions, log, toasts, notice, now };
}

const texts = (t: ReturnType<typeof createToasts>) => t.shown.get().map((x) => x.text);
const prepared = (log: string[]) => (slug: string) => {
  log.push(`prepare ${slug}`);
  return { escape: () => log.push('escape'), done: () => log.push('done'), away: () => log.push('away') };
};

describe(createPageActions, () => {
  it('says a star or a pin with a toast before the list is read, and a refusal on the error line alone', async () => {
    expect.assertions(4);
    const ok = made();
    ok.notice.say('before');
    await ok.actions.toggleFavorite(page);
    await ok.actions.togglePin(page);
    expect(ok.log).toStrictEqual([
      'patch a {"favorite":true}',
      'load after 1 toasts',
      'task a {"pin":true}',
      'load after 2 toasts',
    ]);
    // A pin is a task's write, which leaves the error line to the page's writes
    expect(ok.notice.said.get().message).toBe('before');
    const refused = made({ refuse: 'no' });
    await refused.actions.togglePin(page);
    expect(refused.toasts.shown.get()).toStrictEqual([]);
    expect(refused.notice.said.get().message).toBe('ui_save_failed:no');
  });

  it('archives once for two presses, with a way back that takes the focus from the keys, taken before the list', async () => {
    expect.assertions(3);
    const { actions, log, toasts } = made();
    const call = { focus: true, prepare: prepared(log), restoreFocus: () => log.push('restore') };
    await Promise.all([actions.setArchived(page, true, call), actions.setArchived(page, true, call)]);
    expect(log).toStrictEqual(['patch a {"archived":true}', 'prepare a', 'load after 1 toasts']);
    expect(toasts.shown.get()[0]).toMatchObject({ focus: true, button: { undo: true, holds: false, slug: 'a' } });
    buttonOf(toasts.shown.get()[0])?.onEscape?.();
    expect([texts(toasts), log.at(-1)]).toStrictEqual([[], 'escape']);
  });

  it('restores from the way back, which goes, the focus given back after the list if the way back had it', async () => {
    expect.assertions(2);
    const { actions, log, toasts } = made();
    const call = { prepare: prepared(log), restoreFocus: () => log.push('restore') };
    await actions.setArchived(page, true, call);
    const way = toasts.shown.get()[0];
    toasts.focused.set(Number(way?.id));
    await buttonOf(way)?.onPress();
    expect(texts(toasts)).toStrictEqual(['ui_unarchive_done:A']);
    expect(log.slice(-3)).toStrictEqual(['patch a {"archived":false}', 'load after 1 toasts', 'restore']);
  });

  it('archives, restores and archives again before the reads end', async () => {
    expect.assertions(1);
    const { actions, log, toasts } = made({ slow: true });
    const ended = () => new Promise((go) => setTimeout(go, 5));
    const call = { prepare: prepared(log), restoreFocus: () => undefined };
    void actions.setArchived(page, true, call);
    await ended();
    void buttonOf(toasts.shown.get()[0])?.onPress();
    await ended();
    void actions.setArchived(page, true, call);
    await ended();
    // Written, and not read yet: asked already still
    void actions.setArchived(page, true, call);
    await ended();
    expect(log.filter((x) => x.startsWith('patch'))).toStrictEqual([
      'patch a {"archived":true}',
      'patch a {"archived":false}',
      'patch a {"archived":true}',
    ]);
  });

  it('stars, pins and takes off its task once before the reads end, and takes off again once put back', async () => {
    expect.assertions(4);
    const ended = () => new Promise((go) => setTimeout(go, 5));
    const call = (log: string[]) => ({ prepare: prepared(log) });
    const presses: [string, (x: ReturnType<typeof made>) => Promise<void>][] = [
      ['patch a {"favorite":true}', ({ actions }) => actions.toggleFavorite(page)],
      ['task a {"pin":true}', ({ actions }) => actions.togglePin(page)],
      ['task a {"task":"","category":""}', ({ actions, log }) => actions.takePageOff(page, call(log))],
    ];
    for (const [write, press] of presses) {
      const x = made({ slow: true });
      void press(x);
      await ended();
      // Written, and not read yet: asked already still
      void press(x);
      await ended();
      expect(x.log.filter((line) => /^(patch|task)/.test(line))).toStrictEqual([write]);
    }
    const back = made({ slow: true });
    void back.actions.takePageOff(page, call(back.log));
    await ended();
    void buttonOf(back.toasts.shown.get()[0])?.onPress();
    await ended();
    void back.actions.takePageOff(page, call(back.log));
    await ended();
    void back.actions.takePageOff(page, call(back.log));
    await ended();
    expect(back.log.filter((line) => /^(task|put)/.test(line))).toStrictEqual([
      'task a {"task":"","category":""}',
      'put #1 designs {"new":false,"rev":7}',
      'task a {"task":"","category":""}',
    ]);
  });

  it('leaves the way back of an archive in place when restoring fails, and says why', async () => {
    expect.assertions(2);
    const { actions, toasts, notice, now } = made();
    await actions.setArchived(page, true, { prepare: prepared([]), restoreFocus: () => undefined });
    now.refuse = 'no';
    await buttonOf(toasts.shown.get()[0])?.onPress();
    expect(notice.said.get().message).toBe('ui_unarchive_failed:A');
    expect(texts(toasts)).toStrictEqual(['ui_archive_done:A']);
  });

  it('takes a page off its task, its place taken first, with a way back of the task as seen, the list read once', async () => {
    expect.assertions(3);
    const { actions, log, toasts } = made();
    await actions.takePageOff(page, { focus: true, prepare: prepared(log) });
    expect(log).toStrictEqual(['prepare a', 'task a {"task":"","category":""}', 'load after 1 toasts']);
    expect(toasts.shown.get()[0]?.focus).toBe(true);
    await buttonOf(toasts.shown.get()[0])?.onPress();
    expect(log.slice(3)).toStrictEqual(['put #1 designs {"new":false,"rev":7}', 'load after 1 toasts', 'done']);
  });

  it('reads the tasks again in a page whole before the way back reads its task', async () => {
    expect.assertions(1);
    const { actions, log } = made({ screen: 'frame' });
    await actions.takePageOff(page, { prepare: prepared(log) });
    expect(log).toStrictEqual(['prepare a', 'task a {"task":"","category":""}', 'reread tasks', 'load after 1 toasts']);
  });

  it('puts a page back on a task with no row as new, says a stale way back as the store does, and a refused take-off', async () => {
    expect.assertions(3);
    const fresh = made({ known: false, stale: true });
    await fresh.actions.takePageOff(page, { prepare: prepared(fresh.log) });
    await buttonOf(fresh.toasts.shown.get()[0])?.onPress();
    expect(fresh.log).toContain('put #1 designs {"new":true}');
    expect(fresh.notice.said.get().message).toBe('ui_save_failed:stale');
    const refused = made({ refuse: 'no' });
    await refused.actions.takePageOff(page, { prepare: prepared(refused.log) });
    expect(refused.notice.said.get().message).toBe('ui_save_failed:no');
  });

  it('takes a page off for a modal, saying and reading nothing: the reason, or the toast its way back is in', async () => {
    expect.assertions(4);
    const ok = made();
    const off = await ok.actions.takeOff(page, prepared(ok.log), { focus: false });
    expect(off).toStrictEqual({ reason: '', toast: ok.toasts.shown.get()[0]?.id });
    expect(ok.log).toStrictEqual(['prepare a', 'task a {"task":"","category":""}']);
    const refused = made({ refuse: 'no' });
    await expect(refused.actions.takeOff(page, prepared(refused.log))).resolves.toStrictEqual({ reason: 'no' });
    expect(refused.notice.said.get().message).toBe('');
  });

  it('goes back from a page whole deleted, with no toast and no read', async () => {
    expect.assertions(2);
    const { actions, log, toasts } = made({ framed: 'a' });
    await actions.removePage(page, () => log.push('removed'));
    expect(log).toStrictEqual(['delete a', 'removed', 'go /back']);
    expect(toasts.shown.get()).toStrictEqual([]);
  });

  it('says a page deleted from the list and reads the list after', async () => {
    expect.assertions(1);
    const { actions, log, toasts } = made();
    await actions.removePage(page, () => log.push('removed'));
    expect([log, texts(toasts)]).toStrictEqual([['delete a', 'removed', 'load after 1 toasts'], ['ui_delete_done:A']]);
  });

  it('says a page marked unread when it was sent, as the page’s toast for the page shown, the focus read first', async () => {
    expect.assertions(3);
    const shown = made({ shown: 'a' });
    const before = shown.toasts.toast('read', {
      ms: 8000,
      page: true,
      button: { label: 'x', undo: false, holds: false, onPress: () => undefined },
    });
    shown.toasts.focused.set(before);
    await shown.actions.markUnreadSaid('a', () => shown.log.push('focus'));
    expect([texts(shown.toasts), shown.toasts.shown.get()[0]?.page, shown.log]).toStrictEqual([
      ['ui_unread_done:undefined'],
      true,
      ['note cleared', 'focus'],
    ]);
    // Of a page not shown, the words over the page shown stay
    const dropped = made({ outcome: { kind: 'dropped' } });
    await dropped.actions.markUnreadSaid('a', () => undefined);
    expect([dropped.toasts.shown.get(), dropped.log]).toStrictEqual([[], []]);
    const archived = made({ outcome: null });
    await archived.actions.markUnreadSaid('a', () => undefined);
    expect(archived.toasts.shown.get()).toStrictEqual([]);
  });

  it('copies a path and says so, and says a refusal of the clipboard', async () => {
    expect.assertions(3);
    const { actions, toasts, notice } = made();
    await expect(actions.copyPath('/tmp/a')).resolves.toBe(true);
    await expect(actions.copyPath('')).resolves.toBe(false);
    expect([texts(toasts), notice.said.get().message]).toStrictEqual([
      ['ui_path_copied:undefined'],
      'ui_copy_failed:denied',
    ]);
  });
});
