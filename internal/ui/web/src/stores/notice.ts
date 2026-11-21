import { createStore, type Store } from '@/lib/store';

// Who said a mistake: '' an action, 'load' or 'read' (a read mark)
export type From = '' | 'load' | 'read';

export interface Said {
  message: string;
  from: From;
}

export interface Notice {
  said: Store<Said>;
  // say shows message from from, or takes back from's with ''. Each takes back only its own, and only an action writes
  // over an action's, so that one is never lost unread to a load or a mark said again and again
  say: (message: string, from?: From) => void;
}

// createNotice holds the page's one mistake shown
// LIMIT: one place: a load's or a mark's mistake waits behind an action's until the next action. Give each its own if
// that comes to matter
export function createNotice(): Notice {
  const said = createStore<Said>({ message: '', from: '' });
  return {
    said,
    say: (message, from = '') => {
      const now = said.get();
      if (message ? from !== '' && now.from === '' && now.message !== '' : now.from !== from) return;
      const next: From = message ? from : '';
      if (now.message !== message || now.from !== next) said.set({ message, from: next });
    },
  };
}

export const notice = createNotice();
