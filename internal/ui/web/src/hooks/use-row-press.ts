import type { KeyboardEvent, MouseEvent } from 'react';
import { useMemo } from 'react';

import { oneColumn } from '@/lib/platform';
import { menus as pageMenus, type Menus } from '@/stores/menus';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';
import type { Page } from '@/types/page';
import { composing } from '@/utils/keys';
import { pageHref } from '@/utils/page-urls';
import { enterOpens, orderOf, rowPress } from '@/utils/row-press';

// A row's page: an entry's, or a page of a folder's tree (its file)
export interface RowPage {
  readonly slug: string;
  readonly file: string;
}

export interface RowPresses {
  // press is a press on the row's title; key, a key on it
  press: (x: RowPage, e: MouseEvent<HTMLAnchorElement>) => void;
  key: (x: RowPage, e: KeyboardEvent<HTMLAnchorElement>) => void;
}

export interface RowPressDeps {
  // selecting tells rows selected; drawn, the entries drawn, in their order, as the press finds them
  readonly selecting: () => boolean;
  readonly drawn: () => readonly Page[];
  readonly openPage?: OpenPage;
  readonly order?: Order;
  readonly menus?: Menus;
  readonly narrow?: () => boolean;
  readonly assign?: (url: string) => void;
}

// rowPresses is what a row's title does as pressed and as Enter lands on it (ui.html openRow, 6937): beside the list,
// or whole, the rows drawn kept for j and k and the way back. What is open beside the list is read as it is pressed,
// so that a double press goes whole though the list is not drawn anew between the two
export function rowPresses({
  selecting,
  drawn,
  openPage = pageOpen,
  order = pageOrder,
  menus = pageMenus,
  narrow = oneColumn,
  assign = (url) => {
    window.location.assign(url);
  },
}: RowPressDeps): RowPresses {
  const keep = () => {
    const { order: pages, shown } = orderOf(drawn());
    order.keepRows(pages, shown);
  };
  // Whole: the row open beside the list at the version it shows, any other by its link
  const whole = (x: RowPage, link: HTMLAnchorElement) => {
    const picked = openPage.picked.get();
    assign(picked && openPage.isPicked(x) ? pageHref(picked) : link.href);
  };
  return {
    press: (x, e) => {
      const at = rowPress(e, {
        picked: openPage.isPicked(x),
        hasHref: e.currentTarget.hasAttribute('href'),
        menuOpen: menus.open.get() !== null,
        selecting: selecting(),
        oneColumn: narrow(),
      });
      if (at === 'link') return;
      if (at === 'one-column') {
        keep();
        return;
      }
      e.preventDefault();
      if (at === 'again') {
        keep();
        whole(x, e.currentTarget);
      } else if (!openPage.isPicked(x)) openPage.picked.set({ slug: x.slug, file: x.file, v: 0 });
    },
    key: (x, e) => {
      const link = e.currentTarget;
      const opens = enterOpens(e, {
        hasHref: link.hasAttribute('href'),
        expandable: link.hasAttribute('aria-expanded'),
        composing: composing(e.nativeEvent),
      });
      if (!opens) return;
      e.preventDefault();
      keep();
      whole(x, link);
    },
  };
}

// useRowPress is rowPresses, the same through redraws while its deps are, so that a row drawn once (memo) stays
export const useRowPress = (deps: RowPressDeps): RowPresses =>
  // oxlint-disable-next-line react-hooks/exhaustive-deps -- its deps are the fields of deps, not the object made anew
  useMemo(() => rowPresses(deps), [deps.selecting, deps.drawn, deps.openPage, deps.order, deps.menus]);
