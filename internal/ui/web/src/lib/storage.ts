// A browser storage as the page keeps the viewer's things in it: one unavailable, or that refuses (a full one, a
// private window's), keeps nothing, and the page goes on without it
export interface Kept {
  read: (key: string) => string | null;
  write: (key: string, value: string) => void;
  remove: (key: string) => void;
}

// kept wraps the storage which: called at each use, as reaching window.localStorage may itself throw
export function kept(which: () => Storage): Kept {
  return {
    read: (key) => {
      try {
        return which().getItem(key);
      } catch {
        return null;
      }
    },
    write: (key, value) => {
      try {
        which().setItem(key, value);
      } catch {
        // Kept for now only
      }
    },
    remove: (key) => {
      try {
        which().removeItem(key);
      } catch {
        // Nothing kept to forget
      }
    },
  };
}

// The page's two: the browser's (kept across tabs) and the tab's
export const local = kept(() => window.localStorage);
export const session = kept(() => window.sessionStorage);

// readJSON is what the storage holds at key as JSON, or null for none or what is not JSON
export function readJSON(storage: Kept, key: string): unknown {
  const text = storage.read(key);
  if (text === null) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}
