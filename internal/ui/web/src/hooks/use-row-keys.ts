import { useMemo } from 'react';

import type { KeyTable } from '@/hooks/use-keys';
import { oneColumn } from '@/lib/platform';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { rowOpens, type MovedTo } from '@/utils/row-press';

export interface RowKeyDeps {
  readonly pages?: Pages | undefined;
  readonly openPage?: OpenPage | undefined;
  // move is focus-places' moveRow, given by the app that makes the table of keys
  readonly move: (step: number) => MovedTo | null;
  // tick is focus-places' tickRow, x's: left out, the table has no x (a screen of no boxes)
  readonly tick?: (() => void) | undefined;
  readonly narrow?: (() => boolean) | undefined;
}

// rowKeys is the keys that move through the rows of any screen, ↑ ↓ and j k: the focus to the next row's title, its
// page beside the list where it has one
export function rowKeys({
  pages = pagePages,
  openPage = pageOpen,
  move,
  narrow = oneColumn,
  tick,
}: RowKeyDeps): KeyTable {
  const step = (by: number) => () => {
    const to = move(by);
    if (!to) return;
    const page = pages.state.get().entries.find((a) => a.slug === to.slug);
    if (rowOpens({ oneColumn: narrow(), page, dir: to.dir, file: to.file }) && !openPage.isPicked(to))
      openPage.picked.set({ slug: to.slug, file: to.file, v: 0 });
  };
  return { ArrowDown: step(1), ArrowUp: step(-1), j: step(1), k: step(-1), ...(tick ? { x: tick } : {}) };
}

// useRowKeys is rowKeys for the screen's table of keys (useKeys), which the app makes (P8)
export function useRowKeys({ pages, openPage, move, narrow, tick }: RowKeyDeps): KeyTable {
  return useMemo(() => rowKeys({ pages, openPage, move, narrow, tick }), [pages, openPage, move, narrow, tick]);
}
