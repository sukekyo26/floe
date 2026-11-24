import { getBoot, type Boot } from '@/config/boot';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';
import { folders as openFolders, type Folders } from '@/stores/open-folders';
import { openPage as page, type OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';
import { pages as allPages, type Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import { arrange, kindOf } from '@/utils/list-view';
import { pageHref, type PageAt } from '@/utils/page-urls';

export interface FrameMoves {
  // moveFrame goes to the next (1) or the one before (-1) of the pages j and k go through; at an end, nowhere
  moveFrame: (step: 1 | -1) => void;
}

interface Deps {
  boot?: () => Boot;
  order?: Order;
  listQuery?: ListQuery;
  pages?: Pages;
  folders?: Pick<Folders, 'side'>;
  openPage?: Pick<OpenPage, 'frameAt'>;
  go?: (href: string) => void;
}

export function createFrameMoves({
  boot = getBoot,
  order = pageOrder,
  listQuery = pageQuery,
  pages = allPages,
  folders = openFolders,
  openPage = page,
  go = (href) => {
    window.location.assign(href);
  },
}: Deps = {}): FrameMoves {
  // The order first found is kept, though what was kept goes or the list read changes since
  let first: string[] | null = null;
  // The pages of the list that opened the frame, the ones gone since left out. Opened otherwise (from the URL, the
  // search), the list it was last on, or if it is not on that all entries of its kind
  function inOrder(a: Page, entries: readonly Page[]): string[] {
    const kept = order.keptOrder();
    if (kept?.order.includes(a.slug)) first = kept.order;
    if (!first) {
      const { seen } = listQuery.query.get();
      const openable = (search: string, over: Partial<ReturnType<ListQuery['readState']>> = {}) =>
        arrange(entries, { ...listQuery.readState(search), ...over }, seen).filter((x) => !x.missing && !x.folder);
      let found = openable(listQuery.savedList());
      if (!found.some((x) => x.slug === a.slug))
        found = openable('', { kind: kindOf(a), view: a.archived ? 'archived' : 'all' });
      first = found.map((x) => x.slug);
    }
    return first.filter((slug) => entries.some((x) => x.slug === slug && !x.missing));
  }
  return {
    moveFrame: (step) => {
      const at = openPage.frameAt();
      const { entries } = pages.state.get();
      const a = entries.find((x) => x.slug === boot().framed);
      if (!at || !a) return;
      // A folder's page goes through the pages of its directory, by name
      const list: PageAt[] = at.file
        ? (folders.side.get().fileListing?.files ?? []).map((f) => ({ slug: a.slug, file: f.file }))
        : inOrder(a, entries).map((slug) => ({ slug }));
      const here = list.findIndex((x) => x.slug === at.slug && (x.file ?? '') === at.file);
      const next = here === -1 ? undefined : list[here + step];
      if (next) go(pageHref(next));
    },
  };
}

export const frameMoves = createFrameMoves();
