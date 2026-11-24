import { local, readJSON, type Kept } from '@/lib/storage';
import { createStore, type Store } from '@/lib/store';

// A side column's width and whether it shows, the viewer's, kept in the browser
export interface Side {
  width: number;
  open: boolean;
}

export interface KeptSide {
  side: () => Store<Side>;
  setWidth: (width: number) => void;
  setOpen: (open: boolean) => void;
  // keep writes what is shown now into the browser: as a drag or a key stops, and as it opens or closes
  keep: () => void;
}

// createKeptSide is the side column of key, width wide until the viewer sets one. What the browser holds is read at the
// first call, never as the module loads; one unreadable, or of another shape, leaves the default
export function createKeptSide(key: string, width: number, { storage = local }: { storage?: Kept } = {}): KeptSide {
  let kept: Store<Side> | null = null;
  const side = () => {
    if (kept === null) {
      const start: Side = { width, open: true };
      const saved = readJSON(storage, key) as Partial<Record<keyof Side, unknown>> | null;
      if (Number.isFinite(saved?.width)) start.width = saved?.width as number;
      if (typeof saved?.open === 'boolean') start.open = saved.open;
      kept = createStore(start);
    }
    return kept;
  };
  return {
    side,
    setWidth: (next) => {
      side().set({ ...side().get(), width: next });
    },
    setOpen: (open) => {
      side().set({ ...side().get(), open });
    },
    keep: () => {
      storage.write(key, JSON.stringify(side().get()));
    },
  };
}
