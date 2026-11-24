import { client as pageClient, type Client } from '@/lib/api-client';
import { readListing } from '@/lib/pages';
import { local, type Kept } from '@/lib/storage';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import { sameListings } from '@/utils/listings';
import { dirOf, openKey } from '@/utils/paths';
import type { Listing, Page } from '@/types/page';

// What is open of the folders in the list, and of the tree on the frame's left of a folder's page, with each directory
// as read (GET /api/pages/{slug}/files): an Error for one that could not be read. The two differ in what they key,
// keep and do with a directory gone, so each has its own state.
export interface Folders {
  // The list's: open folders and their directories by openKey (slug, or slug:dir), kept in the browser (list.open)
  list: Store<{ opened: ReadonlySet<string>; listings: ReadonlyMap<string, Listing | Error> }>;
  // The frame's tree, of the one folder framed: directories open ('' the top), kept nowhere; fileListing is the
  // directory of the page framed, for j and k
  side: Store<{ opened: ReadonlySet<string>; listings: ReadonlyMap<string, Listing | Error>; fileListing?: Listing }>;
  // readOpen reads the directories open of the folders in list (the entries as just read). It changes nothing: it
  // returns what puts its answers in, for the read of the list to call as it takes that list (true when they changed),
  // or null when a later read began
  readOpen: (list: readonly Page[], signal: AbortSignal) => Promise<(() => boolean) | null>;
  // readSide reads the tree's directories open, each under ones open, and the page's own, whatever is open; as readOpen
  readSide: (slug: string, file: string, signal: AbortSignal) => Promise<(() => boolean) | null>;
  // takeOpen takes what is open from open= of the URL and from list.open, once: before anything writes the URL, which
  // drops open= (stores/list-query's sync calls it first)
  takeOpen: () => void;
  // toggle opens or closes the directory dir ('' for the top) of the folder slug in the list; closed, what is open
  // under it closes too
  toggle: (slug: string, dir: string) => Promise<void>;
  // toggleSide opens or closes the directory dir of the frame's tree of the folder slug, whose page file is framed;
  // closed, what is open under it closes too, the page's own directory still read for j and k
  toggleSide: (slug: string, file: string, dir: string) => Promise<void>;
}

const OPEN_KEY = 'list.open';

interface Deps {
  client?: Client;
  notice?: Notice;
  storage?: Kept;
  // The page's query, of which open= opens folders once (a folder's URL /a/{slug}/ leads to the list so)
  search?: () => string;
}

// slugDir splits an openKey: a slug has no colon
const slugDir = (key: string): [string, string] => {
  const at = key.indexOf(':');
  return at < 0 ? [key, ''] : [key.slice(0, at), key.slice(at + 1)];
};

export function createFolders({
  client = pageClient,
  notice = pageNotice,
  storage = local,
  search = () => window.location.search,
}: Deps = {}): Folders {
  const list = createStore<{ opened: ReadonlySet<string>; listings: ReadonlyMap<string, Listing | Error> }>({
    opened: new Set(),
    listings: new Map(),
  });
  const side = createStore<{
    opened: ReadonlySet<string>;
    listings: ReadonlyMap<string, Listing | Error>;
    fileListing?: Listing;
  }>({ opened: new Set(), listings: new Map() });
  // The last read of each begun, whose answers alone are taken: an earlier one, slower, would take away what was opened since
  let openReads = 0;
  let sideReads = 0;
  // What is open is taken from the URL and the browser at the first call, never as the module loads
  let started = false;
  let sideStarted = false;
  const keep = (opened: ReadonlySet<string>) => {
    storage.write(OPEN_KEY, JSON.stringify([...opened]));
  };
  const start = () => {
    if (started) return;
    started = true;
    const opened = new Set(new URLSearchParams(search()).getAll('open'));
    const text = storage.read(OPEN_KEY);
    try {
      const was: unknown = text === null ? null : JSON.parse(text);
      if (Array.isArray(was)) for (const key of was) if (typeof key === 'string') opened.add(key);
    } catch {
      // What the storage holds is not ours
    }
    keep(opened);
    list.set({ opened, listings: list.get().listings });
  };

  async function readOpen(entries: readonly Page[], signal: AbortSignal) {
    start();
    const n = ++openReads;
    const { opened } = list.get();
    // A folder no longer registered leaves what is kept
    const kept = new Set([...opened].filter((key) => entries.some((x) => x.folder && x.slug === slugDir(key)[0])));
    const shown = (key: string) => {
      const [slug, dir] = slugDir(key);
      const a = entries.find((x) => x.slug === slug);
      if (!a?.folder || a.missing || !kept.has(slug)) return false;
      const parts = dir ? dir.split('/') : [];
      return parts.slice(0, -1).every((_, i) => kept.has(openKey(slug, parts.slice(0, i + 1).join('/'))));
    };
    const keys = [...kept].filter(shown);
    const read = await Promise.all(keys.map((key) => readListing(client, ...slugDir(key), signal)));
    if (n !== openReads) return null;
    return () => {
      const now = list.get();
      const before = sameListings(now.listings);
      const next = new Set([...now.opened].filter((key) => kept.has(key)));
      const listings = new Map<string, Listing | Error>();
      keys.forEach((key, i) => {
        const l = read[i];
        if (l === null || l === undefined) next.delete(key);
        else listings.set(key, l);
      });
      if (next.size !== now.opened.size) keep(next);
      const changed = sameListings(listings) !== before;
      if (changed || next.size !== now.opened.size) list.set({ opened: next, listings });
      return changed;
    };
  }

  async function readSide(slug: string, file: string, signal: AbortSignal) {
    if (!sideStarted) {
      sideStarted = true;
      // The directories the page is under are open as it opens
      const parts = dirOf(file).split('/');
      side.set({ ...side.get(), opened: new Set([...parts.map((_, i) => parts.slice(0, i).join('/')), dirOf(file)]) });
    }
    const n = ++sideReads;
    const { opened } = side.get();
    const keys = [...new Set([...opened].filter((dir) => !dir || opened.has(dirOf(dir))).concat(dirOf(file)))];
    const read = await Promise.all(keys.map((dir) => readListing(client, slug, dir, signal)));
    if (n !== sideReads) return null;
    return () => {
      const now = side.get();
      const before = sameListings(now.listings);
      // One gone for now stays open, to show again when it is back
      const listings = new Map<string, Listing | Error>();
      keys.forEach((dir, i) => {
        const l = read[i];
        if (l !== null && l !== undefined) listings.set(dir, l);
      });
      const own = listings.get(dirOf(file));
      const changed = sameListings(listings) !== before;
      if (changed)
        side.set({ opened: now.opened, listings, ...(own instanceof Error || !own ? {} : { fileListing: own }) });
      return changed;
    };
  }

  async function toggle(slug: string, dir: string) {
    start();
    const key = openKey(slug, dir);
    const now = list.get();
    if (now.opened.has(key)) {
      const under = (k: string) => k === key || k.startsWith(dir ? `${key}/` : `${slug}:`);
      const opened = new Set([...now.opened].filter((k) => !under(k)));
      const listings = new Map([...now.listings].filter(([k]) => !under(k)));
      keep(opened);
      list.set({ opened, listings });
      return;
    }
    const opened = new Set(now.opened).add(key);
    keep(opened);
    list.set({ opened, listings: now.listings });
    // A read of all open begun before this one would take them without it
    openReads++;
    let l: Listing | Error | null;
    try {
      l = await readListing(client, slug, dir);
    } catch (err) {
      // The load's: it stays open, and the next look reads it again
      notice.say(t('ui_load_failed', err instanceof Error ? err.message : String(err)), 'load');
      return;
    }
    const after = list.get();
    // Closed while it was read
    if (!after.opened.has(key)) return;
    if (l === null) {
      const shut = new Set(after.opened);
      shut.delete(key);
      keep(shut);
      list.set({ opened: shut, listings: after.listings });
      return;
    }
    list.set({ opened: after.opened, listings: new Map(after.listings).set(key, l) });
  }

  async function toggleSide(slug: string, file: string, dir: string) {
    const now = side.get();
    if (now.opened.has(dir)) {
      const under = (k: string) => k === dir || k.startsWith(`${dir}/`);
      side.set({
        ...now,
        opened: new Set([...now.opened].filter((k) => !under(k))),
        listings: new Map([...now.listings].filter(([k]) => !under(k) || k === dirOf(file))),
      });
      return;
    }
    side.set({ ...now, opened: new Set(now.opened).add(dir) });
    // A read of all open begun before this one would take them without it
    sideReads++;
    let l: Listing | Error | null;
    try {
      l = await readListing(client, slug, dir);
    } catch (err) {
      // The load's: it stays open, and the next look reads it again
      notice.say(t('ui_load_failed', err instanceof Error ? err.message : String(err)), 'load');
      return;
    }
    const after = side.get();
    // Closed while it was read; one gone for now stays open, as readSide's
    if (!after.opened.has(dir) || l === null) return;
    const listings = new Map(after.listings).set(dir, l);
    // The page's own directory read anew is what j and k go through, as readSide's
    if (dir !== dirOf(file)) side.set({ ...after, listings });
    else side.set({ opened: after.opened, listings, ...(l instanceof Error ? {} : { fileListing: l }) });
  }

  return { list, side, readOpen, readSide, takeOpen: start, toggle, toggleSide };
}

export const folders = createFolders();
