import { createStore, type Store } from '@/lib/store';

export interface Menus {
  // open is the one menu open (its id), or null: one opens at a time
  open: Store<string | null>;
  show: (id: string) => void;
  // hide closes the menu id, if it is the one open
  hide: (id: string) => void;
}

export function createMenus(): Menus {
  const open = createStore<string | null>(null);
  return {
    open,
    show: (id) => {
      open.set(id);
    },
    hide: (id) => {
      if (open.get() === id) open.set(null);
    },
  };
}

export const menus = createMenus();
