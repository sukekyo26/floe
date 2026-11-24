import { client as pageClient, type Client } from '@/lib/api-client';
import { createStore, type Store } from '@/lib/store';
import { PAGES_API } from '@/utils/page-urls';

// The words over a page newly at another version than the one read: the page, the version read before, and how many
// places changed since
export interface Note {
  slug: string;
  read: number;
  changed: number;
}

export interface PageNote {
  note: Store<Note | null>;
  // to is the version the words were asked for, null for none
  to: () => number | null;
  // show asks what changed since the version read before n, and says it if anything did; an answer come after the words
  // were cleared or asked again is dropped
  show: (slug: string, n: number) => Promise<void>;
  // clear takes the words away: another page loaded, this one marked unread or drawn anew at another version
  clear: () => void;
}

// createPageNote shows and clears the note of a page, shared as marking a page unread (stores/page-actions) clears it
// too. The words come only once the page was read, as what was read before is known then
// LIMIT: all of /diff is read for its counts. Add an answer of the counts alone if that comes to weigh
export function createPageNote({ client = pageClient }: { client?: Client } = {}): PageNote {
  const note = createStore<Note | null>(null);
  let to: number | null = null;
  let turn = 0;
  const clear = () => {
    to = null;
    turn++;
    note.set(null);
  };
  return {
    note,
    to: () => to,
    clear,
    show: async (slug, n) => {
      clear();
      to = n;
      const asked = turn;
      try {
        const base = `${PAGES_API}/${encodeURIComponent(slug)}`;
        const { read } = (await (await client.api(`${base}/versions`)).json()) as { read?: number };
        if (!read || read >= n) return;
        const res = await client.api(`${base}/diff?from=${String(read)}&to=${String(n)}`);
        if (!res.ok) return;
        const { counts } = (await res.json()) as { counts: { changed: number; added: number; removed: number } };
        const changed = counts.changed + counts.added + counts.removed;
        if (changed === 0 || asked !== turn) return;
        note.set({ slug, read, changed });
      } catch {
        // Words that help, not needed: the client has told if the server was not reached
      }
    },
  };
}

export const pageNote = createPageNote();
