import type { Listing } from '@/types/page';
import { openKey } from '@/utils/paths';

// A row of a folder's tree in the list, d deep (1 under the folder's row): a directory, a page of it, or a directory's
// note (unread, cut, empty), of its listing. key is the row's, the same through redraws
export type TreeItem =
  | { readonly kind: 'dir'; readonly key: string; readonly d: number; readonly path: string; readonly open: boolean }
  | {
      readonly kind: 'page';
      readonly key: string;
      readonly d: number;
      readonly file: string;
      readonly updatedAt: string;
    }
  | { readonly kind: 'note'; readonly key: string; readonly d: number; readonly listing: Listing | Error };

// noted tells a directory that says something under its rows: unread, cut or empty
const noted = (l: Listing | Error): boolean => l instanceof Error || l.more > 0 || l.dirs.length + l.files.length === 0;

// treeItems is the rows of the directory dir of the folder slug, d deep, in the order drawn: its directories, each
// followed by its own rows when open, then its pages, then its note. A directory not read yet has none until it is
export function treeItems(
  slug: string,
  dir: string,
  d: number,
  listings: ReadonlyMap<string, Listing | Error>,
  opened: ReadonlySet<string>,
): TreeItem[] {
  const l = listings.get(openKey(slug, dir));
  if (l === undefined) return [];
  const note: TreeItem = { kind: 'note', key: `note:${openKey(slug, dir)}`, d, listing: l };
  if (l instanceof Error) return [note];
  const rows: TreeItem[] = [];
  for (const name of l.dirs) {
    const path = dir ? `${dir}/${name}` : name;
    const open = opened.has(openKey(slug, path));
    rows.push({ kind: 'dir', key: `dir:${openKey(slug, path)}`, d, path, open });
    if (open) rows.push(...treeItems(slug, path, d + 1, listings, opened));
  }
  for (const p of l.files)
    rows.push({ kind: 'page', key: `page:${openKey(slug, p.file)}`, d, file: p.file, updatedAt: p.updatedAt });
  if (noted(l)) rows.push(note);
  return rows;
}
