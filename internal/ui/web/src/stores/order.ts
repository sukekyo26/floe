import { getBoot, type Boot } from '@/config/boot';
import { readJSON, session, type Kept } from '@/lib/storage';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';
import type { OpenAt } from '@/stores/open-page';

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
  // keepAt keeps at as the frame the rows that opened it know, which a way back picks; none where it is not of them
  keepAt: (at: OpenAt) => void;
  // renameKept keeps a page renamed whole of the rows that opened it, under its new name
  renameKept: (was: string, now: string) => void;
  // comeBack takes, back on the list that opened a page whole (by its back button or h, which leave pressed, or the
  // browser's own way back: returned), the row it was at, for backToRow; and forgets the order either way. The logo or
  // a URL to the same list is no way back
  comeBack: (returned: boolean) => void;
  // takeCameFrom gives the row come back to, once
  takeCameFrom: () => OpenAt | null;
}

// atOf is the row kept as a page opened whole, made anew of what the browser kept: a slug, a folder's file ('' for
// none) and a version (0 for the latest)
const atOf = (x: unknown): OpenAt | null => {
  const at = x as { slug?: unknown; file?: unknown; v?: unknown } | null;
  if (typeof at?.slug !== 'string') return null;
  return { slug: at.slug, file: typeof at.file === 'string' ? at.file : '', v: Number(at.v) || 0 };
};

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
  // Plain, not a store: read once by backToRow, which nothing draws from
  let cameFrom: OpenAt | null = null;
  function keptOrder(): RowsKept | null {
    const kept = readJSON(storage, ORDER_KEY) as Partial<RowsKept> | null;
    return Array.isArray(kept?.order) && Array.isArray(kept.shown) && typeof kept.back === 'string'
      ? (kept as RowsKept)
      : null;
  }
  const writeKept = (kept: RowsKept) => {
    storage.write(ORDER_KEY, JSON.stringify(kept));
  };
  const editKept = (change: (kept: RowsKept) => Partial<RowsKept>) => {
    const kept = keptOrder();
    if (kept) writeKept({ ...kept, ...change(kept) });
  };
  return {
    keptOrder,
    writeKept,
    editKept,
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
    keepAt: (at) => {
      editKept((kept) => ({ at: kept.shown.includes(boot().framed) ? at : null }));
    },
    comeBack: (returned) => {
      const kept = keptOrder();
      if (kept && (kept.pressed === true || returned) && kept.back === here()) cameFrom = atOf(kept.at);
      storage.remove(ORDER_KEY);
    },
    takeCameFrom: () => {
      const at = cameFrom;
      cameFrom = null;
      return at;
    },
    renameKept: (was, now) => {
      const named = (slug: string) => (slug === was ? now : slug);
      editKept((kept) => {
        const at = kept.at as { slug?: unknown } | undefined;
        return {
          order: kept.order.map(named),
          shown: kept.shown.map(named),
          ...(at && typeof at.slug === 'string' ? { at: { ...at, slug: named(at.slug) } } : {}),
        };
      });
    },
  };
}

export const order = createOrder();
