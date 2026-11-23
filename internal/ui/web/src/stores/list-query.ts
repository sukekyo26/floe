import { getBoot, screenOf, type Boot } from '@/config/boot';
import { session, type Kept } from '@/lib/storage';
import { createStore, type Store } from '@/lib/store';
import { folders as pageFolders, type Folders } from '@/stores/open-folders';
import type { ListState } from '@/utils/list-view';

// The list's state (the shelf's picks and the search) lives in the URL, so a reload opens the same list. It is also kept
// for the tab, for going back to the list from a page; the task list keeps its own, which never holds the list's
const LIST_KEY = 'pages.list';
const TASKS_KEY = 'pages.tasks';
// The task whose page the task list was gone back to from, for the focus as it opens
const FROM_KEY = 'pages.tasks.from';

const KINDS = ['page', 'link', 'folder'];

export interface Query extends ListState {
  // seen are the pages of the view "new" seen since it was picked: read, they stay on it until the view is let go or
  // picked anew, or the page reloads
  seen: ReadonlySet<string>;
}

export interface ListQuery {
  query: Store<Query>;
  // readState reads a query string: the view and picks it names (what the screen has not left out), and open=
  readState: (search: string) => ListState & { open: string[] };
  // filter is the one place the list's filters change: the view and the shelf's picks, as change has them. The
  // selection, which it lets go too, is the page list's to clear
  filter: (change: Partial<Omit<ListState, 'q'>>) => void;
  setWords: (q: string) => void;
  // see keeps the pages of the view "new" as seen, once for all that are new to it
  see: (slugs: readonly string[]) => void;
  // sync writes the state into the URL, defaults (all, no pick, no search) left out, and keeps it for the tab
  sync: () => void;
  savedList: () => string;
  savedTasks: () => string;
  // synced is the URL's query as sync last wrote and kept it: what reads the kept lists draws again as it changes
  synced: Store<string>;
  keepFrom: (key: string) => void;
  // takeFrom is the task kept by keepFrom, once
  takeFrom: () => string | null;
}

interface Deps {
  boot?: () => Boot;
  search?: () => string;
  storage?: Kept;
  // replace writes url as this tab's URL, adding no history
  replace?: (url: string) => void;
  href?: () => string;
  folders?: Pick<Folders, 'takeOpen'>;
}

export function createListQuery({
  boot = getBoot,
  search = () => window.location.search,
  storage = session,
  replace = (url) => {
    window.history.replaceState(null, '', url);
  },
  href = () => window.location.href,
  folders = pageFolders,
}: Deps = {}): ListQuery {
  const tasks = () => boot().screen === 'tasks';
  const views = () => (tasks() ? boot().states.map((s) => s.name) : ['new', 'fav', 'archived']);
  const firstView = () => (tasks() ? (boot().states.find((s) => s.first)?.name ?? '') : 'all');
  // The task list, not a task's page
  const taskList = () => screenOf(boot(), search()) === 'taskList';
  const synced = createStore('');

  function readState(text: string) {
    const p = new URLSearchParams(text);
    if (tasks()) for (const key of ['kind', 'template', 'tag', 'q']) p.delete(key);
    const view = p.get('view') ?? '';
    const kind = p.get('kind') ?? '';
    return {
      view: views().includes(view) ? view : firstView(),
      kind: KINDS.includes(kind) ? kind : '',
      template: p.get('template') ?? '',
      project: p.get('project') ?? '',
      tag: p.get('tag') ?? '',
      q: p.get('q') ?? '',
      open: p.getAll('open'),
    };
  }

  // Read from the URL at the first call, never as the module loads
  let query: Store<Query> | null = null;
  const store = () => {
    if (query === null) {
      const { open: _open, ...state } = readState(search());
      query = createStore<Query>({ ...state, seen: new Set() });
    }
    return query;
  };

  const savedQuery = (key: string) => storage.read(key) ?? '';

  return {
    get query() {
      return store();
    },
    readState,
    filter: (change) => {
      const now = store().get();
      store().set({ ...now, ...change, seen: 'view' in change ? new Set() : now.seen });
    },
    setWords: (q) => {
      store().set({ ...store().get(), q });
    },
    see: (slugs) => {
      const now = store().get();
      const fresh = slugs.filter((slug) => !now.seen.has(slug));
      if (fresh.length > 0) store().set({ ...now, seen: new Set([...now.seen, ...fresh]) });
    },
    sync: () => {
      // open= opens folders once, as the page opens: taken before the URL is written without it
      folders.takeOpen();
      const { seen: _seen, ...now } = store().get();
      const state = { ...now, view: now.view === firstView() ? '' : now.view, q: now.q.trim() };
      const url = new URL(href());
      // Whatever else it has (open=, once taken) goes
      url.search = new URLSearchParams(Object.entries(state).filter(([, value]) => value)).toString();
      replace(url.toString());
      storage.write(taskList() ? TASKS_KEY : LIST_KEY, url.search);
      synced.set(url.search);
    },
    savedList: () => savedQuery(LIST_KEY),
    savedTasks: () => savedQuery(TASKS_KEY),
    synced,
    keepFrom: (key) => {
      storage.write(FROM_KEY, key);
    },
    takeFrom: () => {
      const key = storage.read(FROM_KEY);
      storage.remove(FROM_KEY);
      return key;
    },
  };
}

export const listQuery = createListQuery();
