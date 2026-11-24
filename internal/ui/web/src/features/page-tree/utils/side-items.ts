import type { Listing } from '@/types/page';
import { openKey } from '@/utils/paths';
import { treeItems, type TreeItem } from '@/utils/tree-items';

// sideItems is the rows of the frame's tree of the folder slug in the order drawn, as the list's tree has them: its
// directories ('' the top) opened and as read are keyed as the list's are (openKey), so that one treeItems orders both
export function sideItems(
  slug: string,
  opened: ReadonlySet<string>,
  listings: ReadonlyMap<string, Listing | Error>,
): TreeItem[] {
  const keyed = new Map([...listings].map(([dir, l]) => [openKey(slug, dir), l]));
  return treeItems(slug, '', 0, keyed, new Set([...opened].map((dir) => openKey(slug, dir))));
}
