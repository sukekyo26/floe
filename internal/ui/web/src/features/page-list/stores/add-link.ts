import { client as pageClient, type Client } from '@/lib/api-client';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';
import { NO_PICKS } from '@/features/page-list/utils/list-shelf';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { folders as pageFolders, type Folders } from '@/stores/open-folders';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import { PAGES_API } from '@/utils/page-urls';

export interface AddValues {
  path: string;
  slug: string;
  description: string;
}

const EMPTY: AddValues = { path: '', slug: '', description: '' };

export interface AddLink {
  // values are the fields as typed, kept as the modal closes unsent
  values: Store<AddValues>;
  // send adds the link or the folder, holding the modal while it goes, and closes it once added: whether it was added
  // and, refused, why (words that may be empty), or null for one not sent (another went)
  send: (
    whileHeld: (write: () => Promise<void>) => Promise<boolean>,
    close: () => void,
  ) => Promise<{ added: boolean; reason: string } | null>;
}

interface Deps {
  client?: Client;
  query?: ListQuery;
  folders?: Folders;
  pages?: Pages;
  toasts?: Toasts;
}

// createAddLink is the modal that registers a file or a folder by its path: a field left blank is not sent. Added, the
// list is shown whole, the new one on it (a folder open), read after the modal lets go
export function createAddLink({
  client = pageClient,
  query = listQuery,
  folders = pageFolders,
  pages = pagePages,
  toasts = pageToasts,
}: Deps = {}): AddLink {
  const values = createStore<AddValues>(EMPTY);
  return {
    values,
    send: async (whileHeld, close) => {
      const now = values.get();
      const body: Partial<AddValues> = {};
      for (const key of ['path', 'slug', 'description'] as const) {
        const value = now[key].trim();
        if (value) body[key] = value;
      }
      // What the write answered, set inside the hold
      const got: { added: { slug: string; folder?: boolean } | null; reason: string } = { added: null, reason: '' };
      const ran = await whileHeld(async () => {
        try {
          const res = await client.api(PAGES_API, {
            method: 'POST',
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify(body),
          });
          // The server answers with the reasons as text, a line each
          if (!res.ok) got.reason = t('ui_http_error', await res.text(), res.status);
          else got.added = (await res.json()) as { slug: string; folder?: boolean };
        } catch (e) {
          got.reason = e instanceof Error ? e.message : String(e);
        }
      });
      if (!ran) return null;
      if (!got.added) return { added: false, reason: got.reason };
      const { slug, folder } = got.added;
      values.set(EMPTY);
      close();
      toasts.toast(t(folder ? 'ui_folder_added' : 'ui_link_added', slug));
      query.setWords('');
      // Opened, not closed if it was open already: toggle would close it
      if (folder && !folders.list.get().opened.has(slug)) void folders.toggle(slug, '');
      query.filter({ view: 'all', ...NO_PICKS });
      await pages.load();
      return { added: true, reason: '' };
    },
  };
}

export const addLink = createAddLink();
