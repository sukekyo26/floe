import { getBoot, screenOf, type Boot } from '@/config/boot';
import { client as pageClient, type Client } from '@/lib/api-client';
import { readPages } from '@/lib/pages';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';
import { readTasks, type TasksRead } from '@/lib/tasks';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import { folders as pageFolders, type Folders } from '@/stores/open-folders';
import type { Page } from '@/types/page';
import type { TaskWritten } from '@/types/task';

// The list as last taken: the entries, the tasks (their error only the task screens say), why the last read failed, and
// why the screen failed to draw what it took. read tells a list taken once at least: till then, entries is no list
export interface PagesState {
  read: boolean;
  entries: Page[];
  tasks: TasksRead;
  readWhy: string;
  drawWhy: string;
}

export interface Pages {
  state: Store<PagesState>;
  // load reads the list, one read at a time: one asked while another goes waits for it, and every ask meanwhile joins
  // the one waiting, so no answer overtakes another. polled tells one asked by no action of the person's. It returns
  // once a read begun after the call has been taken, failed, or, a poll's, given way to a press
  load: (polled?: boolean) => Promise<void>;
  // whileReading is the read going, which a read mark waits for, or null
  whileReading: () => Promise<undefined> | null;
  // pressed and released tell a press on the rows (#list, #pin-list) and its release anywhere: a poll's answer meanwhile
  // is not taken, as Chrome sends neither click nor dblclick for a row gone between press and release
  pressed: () => void;
  released: () => void;
  // watch reads the list every POLL_MS, and as the tab shows again, while it shows; it returns what stops that
  watch: () => () => void;
  // takeTasks puts in the tasks a write answered with, so that the next write is of their rev
  takeTasks: (written: readonly TaskWritten[]) => void;
  // rereadTasks reads the tasks alone, for a frame, whose load reads none
  rereadTasks: () => Promise<void>;
  // readRev is the rev of a task as this screen knows it: as read, or as it wrote it since; 0 of no row
  readRev: (project: string, name: string) => number;
  // ended counts the reads of the list ended, for a screen that failed to draw to try again after the next
  ended: Store<number>;
  // drawFailed says why the screen failed to draw ('' once it draws), after a read's mistake and before the tasks'
  drawFailed: (why: string) => void;
}

// What a page is told of the reads of the list, for the browser tests to wait on: the reads begun and ended
export type Counted = (begun: number, ended: number) => void;
const countOnPage: Counted = (begun, ended) => {
  document.documentElement.dataset['loadsBegun'] = String(begun);
  document.documentElement.dataset['loads'] = String(ended);
};

interface Deps {
  client?: Client;
  notice?: Notice;
  folders?: Folders;
  boot?: () => Boot;
  search?: () => string;
  shown?: () => Pick<Document, 'hidden' | 'addEventListener' | 'removeEventListener'>;
  count?: Counted;
  // The reads of a load are cut off together at readMs, to fail it rather than hold every one after it
  readMs?: number;
  // LIMIT: changes to the list and a recovered connection are picked up by reading again every 3 s. If hundreds of
  // entries make that heavy, push only the changes over SSE
  pollMs?: number;
  // LIMIT: a press held longer may lose its click to a new answer
  pressMs?: number;
}

const NO_TASKS: TasksRead = { error: '', tasks: {}, raw: '' };

export function createPages({
  client = pageClient,
  notice = pageNotice,
  folders = pageFolders,
  boot = getBoot,
  search = () => window.location.search,
  shown = () => document,
  count = countOnPage,
  readMs = 10000,
  pollMs = 3000,
  pressMs = 2000,
}: Deps = {}): Pages {
  const state = createStore<PagesState>({ read: false, entries: [], tasks: NO_TASKS, readWhy: '', drawWhy: '' });
  const endedStore = createStore(0);
  // The text of the list and the tasks last taken: an unchanged answer is not taken again
  let taken = '';
  let begun = 0;
  let ended = 0;
  let waiting: { polled: boolean; done?: Promise<void> } | null = null;
  let last: Promise<void> = Promise.resolve();
  let reading: Promise<undefined> | null = null;
  let pressedAt = -Infinity;

  const say = (readWhy: string) => {
    const { tasks, drawWhy } = state.get();
    const why =
      readWhy || drawWhy || (boot().screen === 'tasks' && tasks.error ? t('ui_tasks_failed', tasks.error) : '');
    notice.say(why, 'load');
  };
  const setWhy = (readWhy: string) => {
    if (state.get().readWhy !== readWhy) state.set({ ...state.get(), readWhy });
    say(readWhy);
  };

  // Takes only a list read with no write going or sent meanwhile, which is of the time after every write sent before it
  // answered; read meanwhile, it is read again
  // LIMIT: a write that never answers holds every load, and every read mark after it
  async function readAndTake(polled: boolean): Promise<void> {
    const { screen } = boot();
    const query = new URLSearchParams(search());
    const shown = screenOf(boot(), search());
    let list: { text: string; entries: Page[] };
    let tasks: TasksRead | null;
    let keep: (() => boolean) | null = null;
    const read = Promise.withResolvers<undefined>();
    reading = read.promise;
    try {
      for (;;) {
        // A write sent by what ran on another write's answer goes too
        for (let writes = client.noWrites(); writes; writes = client.noWrites()) await writes;
        const sent = client.writesSent();
        const signal = AbortSignal.timeout(readMs);
        [list, tasks] = await Promise.all([
          readPages(client, signal),
          screen === 'frame' ? null : readTasks(client, signal),
        ]);
        // Held back, but read: the read's mistake is said no more
        if (polled && performance.now() - pressedAt < pressMs) {
          setWhy('');
          return;
        }
        // What is open of the folders, or the frame's tree, taken only with this list
        const file = query.get('file') ?? '';
        if (screen === 'frame' && file) keep = await folders.readSide(boot().framed, file, signal);
        else if (shown === 'list' || shown === 'taskPage') keep = await folders.readOpen(list.entries, signal);
        if (client.writesSent() === sent) break;
      }
    } catch (err) {
      const e = err instanceof Error ? err : new Error(String(err));
      setWhy(
        e.name === 'TimeoutError' ? t('ui_load_slow', readMs / 1000, pollMs / 1000) : t('ui_load_failed', e.message),
      );
      return;
    } finally {
      reading = null;
      read.resolve(undefined);
    }
    keep?.();
    const text = list.text + (tasks?.raw ?? '');
    if (text !== taken) {
      taken = text;
      state.set({ ...state.get(), read: true, entries: list.entries, tasks: tasks ?? state.get().tasks, readWhy: '' });
    }
    setWhy('');
  }

  function load(polled = false): Promise<void> {
    if (waiting) {
      waiting.polled &&= polled;
      return waiting.done ?? Promise.resolve();
    }
    const next: { polled: boolean; done?: Promise<void> } = { polled };
    waiting = next;
    next.done = last
      .then(() => {
        waiting = null;
        count(++begun, ended);
        return readAndTake(next.polled);
      })
      .finally(() => {
        count(begun, ++ended);
        endedStore.set(ended);
      });
    last = next.done.catch(() => undefined);
    return next.done;
  }

  function watch(): () => void {
    const poll = () => {
      if (!shown().hidden) void load(true);
    };
    shown().addEventListener('visibilitychange', poll);
    const timer = setInterval(poll, pollMs);
    return () => {
      shown().removeEventListener('visibilitychange', poll);
      clearInterval(timer);
    };
  }

  function takeTasks(written: readonly TaskWritten[]) {
    if (written.length === 0) return;
    const now = state.get();
    const tasks = { ...now.tasks.tasks };
    for (const w of written) tasks[w.taskProject] = { ...tasks[w.taskProject], [w.task]: w.row };
    state.set({ ...now, tasks: { ...now.tasks, tasks } });
  }

  async function rereadTasks() {
    const tasks = await readTasks(client, AbortSignal.timeout(readMs));
    state.set({ ...state.get(), tasks });
  }

  return {
    state,
    load,
    whileReading: () => reading,
    pressed: () => {
      pressedAt = performance.now();
    },
    released: () => {
      pressedAt = -Infinity;
    },
    watch,
    takeTasks,
    rereadTasks,
    readRev: (project, name) => state.get().tasks.tasks[project]?.[name]?.rev ?? 0,
    ended: endedStore,
    drawFailed: (drawWhy) => {
      // The same said again tells no one: a screen that draws tells it after every draw
      if (state.get().drawWhy === drawWhy) return;
      state.set({ ...state.get(), drawWhy });
      say(state.get().readWhy);
    },
  };
}

export const pages = createPages();
