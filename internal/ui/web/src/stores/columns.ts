import { getBoot, screenOf } from '@/config/boot';
import { local, readJSON, type Kept } from '@/lib/storage';
import { createStore, type Store } from '@/lib/store';
import { openPage as pageOpen, type OpenAt, type OpenPage } from '@/stores/open-page';
import type { Screen } from '@/types/screen';
import { listShown } from '@/utils/column-widths';

// The two columns left of the page, the viewer's, kept in the browser: their widths in px (null for the default, whose
// bounds are in rem) and whether each is open
export interface KeptColumns {
  shelf: number | null;
  list: number | null;
  open: boolean;
  listOpen: boolean;
}

const COLUMNS_KEY = 'list.columns';

export type ColumnName = 'shelf' | 'list';

export interface ColumnsStore {
  columns: Store<KeptColumns>;
  // screen is the screen the columns are laid out on, the one place they read it from
  screen: () => Screen;
  // picked is the row whose page is open beside the list, which a task's page folds the list for
  picked: Store<OpenAt | null>;
  set: (change: Partial<KeptColumns>) => void;
  keep: () => void;
  // show opens or folds the column named and keeps it so
  show: (name: ColumnName, open: boolean) => void;
  // listOpen tells whether the list shows (listShown)
  listOpen: () => boolean;
}

interface Deps {
  screen?: () => Screen;
  storage?: Kept;
  openPage?: OpenPage;
}

export function createColumns({
  screen = () => screenOf(getBoot(), window.location.search),
  storage = local,
  openPage = pageOpen,
}: Deps = {}): ColumnsStore {
  let columns: Store<KeptColumns> | null = null;
  // Read from the browser at the first call, never as the module loads
  const store = () => {
    if (columns === null) {
      const kept: KeptColumns = { shelf: null, list: null, open: true, listOpen: true };
      const saved = readJSON(storage, COLUMNS_KEY) as Partial<Record<keyof KeptColumns, unknown>> | null;
      if (Number.isFinite(saved?.shelf)) kept.shelf = saved?.shelf as number;
      if (Number.isFinite(saved?.list)) kept.list = saved?.list as number;
      if (typeof saved?.open === 'boolean') kept.open = saved.open;
      if (typeof saved?.listOpen === 'boolean') kept.listOpen = saved.listOpen;
      columns = createStore(kept);
    }
    return columns;
  };
  const set = (change: Partial<KeptColumns>) => {
    store().set({ ...store().get(), ...change });
  };
  const keep = () => {
    storage.write(COLUMNS_KEY, JSON.stringify(store().get()));
  };
  return {
    get columns() {
      return store();
    },
    screen,
    picked: openPage.picked,
    set,
    keep,
    show: (name, open) => {
      set(name === 'shelf' ? { open } : { listOpen: open });
      keep();
    },
    listOpen: () => listShown(screen(), store().get().listOpen, openPage.picked.get() !== null),
  };
}

export const columns = createColumns();
