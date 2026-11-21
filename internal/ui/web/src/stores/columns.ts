import { getBoot, screenOf, type Boot } from '@/config/boot';
import { local, readJSON, type Kept } from '@/lib/storage';
import { createStore, type Store } from '@/lib/store';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';

// The two columns left of the page, the viewer's, kept in the browser: null for the default width, in rem, as the widths
// that fold them are, so that a larger text in the browser does not narrow them
export interface Columns {
  shelf: number | null;
  list: number | null;
  open: boolean;
  listOpen: boolean;
}

const COLUMNS_KEY = 'list.columns';

export interface ColumnsStore {
  columns: Store<Columns>;
  set: (change: Partial<Columns>) => void;
  keep: () => void;
  // listOpen tells whether the list shows: the task list has nothing but the list to fold it into, and a task's page
  // keeps its rows until a page is there to take their place
  listOpen: () => boolean;
}

interface Deps {
  boot?: () => Boot;
  search?: () => string;
  storage?: Kept;
  openPage?: OpenPage;
}

export function createColumns({
  boot = getBoot,
  search = () => window.location.search,
  storage = local,
  openPage = pageOpen,
}: Deps = {}): ColumnsStore {
  let columns: Store<Columns> | null = null;
  // Read from the browser at the first call, never as the module loads
  const store = () => {
    if (columns === null) {
      const kept: Columns = { shelf: null, list: null, open: true, listOpen: true };
      const saved = readJSON(storage, COLUMNS_KEY) as Partial<Record<keyof Columns, unknown>> | null;
      if (Number.isFinite(saved?.shelf)) kept.shelf = saved?.shelf as number;
      if (Number.isFinite(saved?.list)) kept.list = saved?.list as number;
      if (typeof saved?.open === 'boolean') kept.open = saved.open;
      if (typeof saved?.listOpen === 'boolean') kept.listOpen = saved.listOpen;
      columns = createStore(kept);
    }
    return columns;
  };
  return {
    get columns() {
      return store();
    },
    set: (change) => {
      store().set({ ...store().get(), ...change });
    },
    keep: () => {
      storage.write(COLUMNS_KEY, JSON.stringify(store().get()));
    },
    listOpen: () => {
      const screen = screenOf(boot(), search());
      return (
        screen === 'taskList' || store().get().listOpen || (screen === 'taskPage' && openPage.picked.get() === null)
      );
    },
  };
}

export const columns = createColumns();
