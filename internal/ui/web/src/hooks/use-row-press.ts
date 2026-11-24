import type { KeyboardEvent, MouseEvent } from 'react';
import { useMemo } from 'react';

import { oneColumn } from '@/lib/platform';
import { menus as pageMenus, type Menus } from '@/stores/menus';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';
import type { Page } from '@/types/page';
import { composing } from '@/utils/keys';
import { pageHref } from '@/utils/page-urls';
import { plainPress } from '@/utils/plain-press';
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
  readonly openPage?: OpenPage | undefined;
  readonly order?: Order | undefined;
  readonly menus?: Menus | undefined;
  readonly narrow?: (() => boolean) | undefined;
  readonly assign?: ((url: string) => void) | undefined;
}

// rowPresses is what a row's title does as pressed and as Enter lands on it: beside the list, or whole, the rows drawn
// kept for j and k and the way back. What is open beside the list is read as it is pressed, so that a double press goes
// whole though the list is not drawn anew between the two
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
export function useRowPress({ selecting, drawn, openPage, order, menus, narrow, assign }: RowPressDeps): RowPresses {
  return useMemo(
    () => rowPresses({ selecting, drawn, openPage, order, menus, narrow, assign }),
    [selecting, drawn, openPage, order, menus, narrow, assign],
  );
}

// pageTitlePress is a press on the title of a page's row, whichever list: a folder's opens and closes its tree, its URL
// leading to the list with it open for a new tab; any other's is the row's press
export function pageTitlePress(
  a: Page,
  presses: RowPresses,
  onFolder: (slug: string) => void,
  e: MouseEvent<HTMLAnchorElement>,
): void {
  if (!a.folder) {
    presses.press({ slug: a.slug, file: '' }, e);
    return;
  }
  if (!plainPress(e)) return;
  e.preventDefault();
  onFolder(a.slug);
}
