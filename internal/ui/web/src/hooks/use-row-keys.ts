import { useMemo } from 'react';

import type { KeyTable } from '@/hooks/use-keys';
import { oneColumn } from '@/lib/platform';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { rowOpens } from '@/utils/row-press';

// The row a key moved to (focus-places' moveRow): its entry's slug, a folder's page's file, a directory or not
export interface MovedTo {
  readonly slug: string;
  readonly file: string;
  readonly dir: boolean;
}

export interface RowKeyDeps {
  readonly pages?: Pages;
  readonly openPage?: OpenPage;
  // move is focus-places' moveRow, given by the app that makes the table of keys
  readonly move: (step: number) => MovedTo | null;
  readonly narrow?: () => boolean;
}

// rowKeys is the keys that move through the rows of any screen, ↑ ↓ and j k (ui.html moveRow): the focus to the next
// row's title, its page beside the list where it has one
export function rowKeys({ pages = pagePages, openPage = pageOpen, move, narrow = oneColumn }: RowKeyDeps): KeyTable {
  const step = (by: number) => () => {
    const to = move(by);
    if (!to) return;
    const page = pages.state.get().entries.find((a) => a.slug === to.slug);
    if (rowOpens({ oneColumn: narrow(), page, dir: to.dir, file: to.file }) && !openPage.isPicked(to))
      openPage.picked.set({ slug: to.slug, file: to.file, v: 0 });
  };
  return { ArrowDown: step(1), ArrowUp: step(-1), j: step(1), k: step(-1) };
}

// useRowKeys is rowKeys for the screen's table of keys (useKeys), which the app makes (P8)
export const useRowKeys = (deps: RowKeyDeps): KeyTable =>
  // oxlint-disable-next-line react-hooks/exhaustive-deps -- its deps are the fields of deps, not the object made anew
  useMemo(() => rowKeys(deps), [deps.pages, deps.openPage, deps.move, deps.narrow]);
