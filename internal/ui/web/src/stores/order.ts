import { getBoot, type Boot } from '@/config/boot';
import { readJSON, session, type Kept } from '@/lib/storage';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';

// What a page open whole goes by: order for j and k, shown for telling that this page is of them, back for where it came
// from, and what the rows that opened it know of it (at, pressed)
export interface RowsKept {
  order: string[];
  shown: string[];
  back: string;
  at?: unknown;
  pressed?: boolean;
}

const ORDER_KEY = 'pages.order';

export interface Order {
  keptOrder: () => RowsKept | null;
  writeKept: (kept: RowsKept) => void;
  editKept: (change: (kept: RowsKept) => Partial<RowsKept>) => void;
  // keepRows keeps the rows as drawn now, of whichever screen
  keepRows: (order: string[], shown: string[]) => void;
  // forgetOrder leaves j and k to the list a page opened otherwise was last on
  forgetOrder: () => void;
  // keptBack is where a page whole goes back to: the screen that opened it, or the page list
  keptBack: () => string;
}

interface Deps {
  boot?: () => Boot;
  storage?: Kept;
  listQuery?: ListQuery;
  // here is this page's path and query
  here?: () => string;
}

export function createOrder({
  boot = getBoot,
  storage = session,
  listQuery = pageQuery,
  here = () => window.location.pathname + window.location.search,
}: Deps = {}): Order {
  function keptOrder(): RowsKept | null {
    const kept = readJSON(storage, ORDER_KEY) as Partial<RowsKept> | null;
    return Array.isArray(kept?.order) && Array.isArray(kept.shown) && typeof kept.back === 'string'
      ? (kept as RowsKept)
      : null;
  }
  const writeKept = (kept: RowsKept) => {
    storage.write(ORDER_KEY, JSON.stringify(kept));
  };
  return {
    keptOrder,
    writeKept,
    editKept: (change) => {
      const kept = keptOrder();
      if (kept) writeKept({ ...kept, ...change(kept) });
    },
    keepRows: (order, shown) => {
      writeKept({ order, shown, back: here() });
    },
    forgetOrder: () => {
      storage.remove(ORDER_KEY);
    },
    keptBack: () => {
      const kept = keptOrder();
      return kept?.shown.includes(boot().framed) ? kept.back : `/${listQuery.savedList()}`;
    },
  };
}

export const order = createOrder();
