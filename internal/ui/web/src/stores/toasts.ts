import type { ReactNode } from 'react';

import { createStore, type Store } from '@/lib/store';

export interface Toast {
  id: number;
  text: string;
  // timed: given a time of its own, for which it stays without fading; page: the one toast of the page shown, which
  // goes as another page is
  timed: boolean;
  page: boolean;
  button: ReactNode;
}

export interface ToastOptions {
  ms?: number;
  page?: boolean;
  button?: ReactNode;
}

export interface Toasts {
  shown: Store<Toast[]>;
  // toast shows text for ms (3 s, fading, when not given); a page's takes the place of the page's before
  toast: (text: string, options?: ToastOptions) => number;
  remove: (id: number) => void;
  // hold keeps a toast with a button while pointed at or focused; again gives it its whole time once more
  hold: (id: number) => void;
  again: (id: number) => void;
}

const SHOWN_MS = 3000;

export function createToasts(): Toasts {
  const shown = createStore<Toast[]>([]);
  const timers = new Map<number, { ms: number; timer: ReturnType<typeof setTimeout> }>();
  let last = 0;

  function remove(id: number) {
    clearTimeout(timers.get(id)?.timer);
    timers.delete(id);
    shown.set(shown.get().filter((x) => x.id !== id));
  }
  function start(id: number, ms: number) {
    clearTimeout(timers.get(id)?.timer);
    timers.set(id, {
      ms,
      timer: setTimeout(() => {
        remove(id);
      }, ms),
    });
  }

  return {
    shown,
    toast: (text, { ms, page = false, button = null } = {}) => {
      if (page) for (const x of shown.get()) if (x.page) remove(x.id);
      const id = ++last;
      shown.set([...shown.get(), { id, text, timed: ms !== undefined, page, button }]);
      start(id, ms ?? SHOWN_MS);
      return id;
    },
    remove,
    hold: (id) => {
      clearTimeout(timers.get(id)?.timer);
    },
    again: (id) => {
      const kept = timers.get(id);
      if (kept) start(id, kept.ms);
    },
  };
}

export const toasts = createToasts();
