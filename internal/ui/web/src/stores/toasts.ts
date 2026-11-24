import { createStore, type Store } from '@/lib/store';

// The button a toast holds, as values Toasts draws it from
export interface ToastButton {
  label: string;
  // name is its whole name (aria-label), which may say whose it is
  name?: string;
  // slug names the page it is of (data-slug), for the way back of a page to be found again
  slug?: string;
  // undo marks the way back of a change (data-testid undo), which tests count apart from any other button
  undo: boolean;
  // holds: the button is disabled while onPress goes (undoToast's), so that one press writes once
  holds: boolean;
  onPress: () => Promise<void> | void;
  // onEscape is what Escape on it does; the key goes no further
  onEscape?: () => void;
}

// The link a toast holds in place of a button: to the task it tells of, its words the toast's
export interface ToastLink {
  label: string;
  href: string;
}

// isToastLink tells a toast's link from its button
export const isToastLink = (x: ToastButton | ToastLink): x is ToastLink => 'href' in x;

// buttonOf is a toast's button, null for none or a link
export const buttonOf = (x: Toast | undefined): ToastButton | null =>
  x?.button && !isToastLink(x.button) ? x.button : null;

// pressToastButton presses a toast's button; one that holds is held (hold(true)) until its press is done
export function pressToastButton(button: ToastButton, hold: (on: boolean) => void): void {
  if (!button.holds) {
    void button.onPress();
    return;
  }
  hold(true);
  void Promise.resolve(button.onPress()).finally(() => {
    hold(false);
  });
}

export interface Toast {
  id: number;
  text: string;
  // timed: given a time of its own, for which it stays without fading; page: the one toast of the page shown, which
  // goes as another page is
  timed: boolean;
  page: boolean;
  button: ToastButton | ToastLink | null;
  // focus: its button takes the focus as it is drawn, once (a change made from the keys)
  focus: boolean;
}

// A toast with a button is given its time: it holds while pointed at or focused, and goes once its time is out
export type ToastOptions =
  | { ms?: number; page?: boolean; button?: undefined; focus?: undefined }
  | { ms: number; page?: boolean; button: ToastButton | ToastLink; focus?: boolean };

export interface Toasts {
  shown: Store<Toast[]>;
  // focused is the toast that has the focus, as its button tells it, or null
  focused: Store<number | null>;
  // toast shows text for ms (3 s, fading, when not given); a page's takes the place of the page's before
  toast: (text: string, options?: ToastOptions) => number;
  // remove takes a toast away, and with it the focus it held, which React may not tell as it goes
  remove: (id: number) => void;
  // hold keeps a toast with a button while pointed at or focused; again gives it its whole time once more
  hold: (id: number) => void;
  again: (id: number) => void;
}

const SHOWN_MS = 3000;

export function createToasts(): Toasts {
  const shown = createStore<Toast[]>([]);
  const focused = createStore<number | null>(null);
  const timers = new Map<number, { ms: number; timer: ReturnType<typeof setTimeout> }>();
  let last = 0;

  function remove(id: number) {
    clearTimeout(timers.get(id)?.timer);
    timers.delete(id);
    if (focused.get() === id) focused.set(null);
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
    focused,
    toast: (text, options = {}) => {
      const { ms, page = false, button = null, focus = false } = options;
      if (page) for (const x of shown.get()) if (x.page) remove(x.id);
      const id = ++last;
      shown.set([...shown.get(), { id, text, timed: ms !== undefined, page, button, focus }]);
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
