import type { Listing } from '@/types/page';

// sameListings writes a tree's directories as read, to tell a change: an Error by its words, which JSON gives none of
export const sameListings = (m: ReadonlyMap<string, Listing | Error>): string =>
  JSON.stringify(
    [...m].sort(([x], [y]) => (x < y ? -1 : Number(x > y))),
    (_, v: unknown) => (v instanceof Error ? `Error ${v.message}` : v),
  );
