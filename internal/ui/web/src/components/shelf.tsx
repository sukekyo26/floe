import { useLayoutEffect, useState, type ReactNode } from 'react';

import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { focusOn } from '@/components/focus-places';
import { FilterIcon } from '@/components/icons';
import { NarrowFilter, type NarrowFilterProps } from '@/components/shelf-filter';
import { t } from '@/lib/t';

// moreIn is where the rest of the shelf is drawn: in the filters' modal while it is open, else in the shelf, which
// hides it on a narrow screen. One place at a time, so that its ids are one each
export const moreIn = (open: boolean): 'dialog' | 'shelf' => (open ? 'dialog' : 'shelf');

// firstChip is what the filters' modal focuses as it opens: the first row that shows, not the box of rows the browser
// takes first where it scrolls, nor an add button, which Enter would open; none for a modal with no row
export const firstChip = <T,>(
  buttons: readonly T[],
  shows: (b: T) => boolean,
  adds: (b: T) => boolean,
): T | undefined => buttons.find((b) => shows(b) && !adds(b));

export interface ShelfProps {
  // views are the view buttons (ViewButton); narrow the way to the filters' words; more the rest of the shelf (its
  // parts and add buttons)
  readonly views: ReactNode;
  readonly narrow: Omit<NarrowFilterProps, 'onOpen'>;
  readonly more: ReactNode;
  // foot is the box at its foot with the button that folds it (the list's columns)
  readonly foot?: ReactNode;
}

function ShelfMore({ inModal, children }: { readonly inModal: boolean; readonly children: ReactNode }) {
  return (
    <div
      id="shelf-more"
      className={`grid grid-cols-[minmax(0,1fr)] gap-[1.1rem] ${inModal ? '-mx-1 min-h-0 overflow-y-auto p-1' : 'wide:hidden'}`}
    >
      {children}
    </div>
  );
}

// Shelf is the column that switches the list, of the page list and of the task list: its views, and the rest of it
// under them, which goes into a modal on a narrow screen with no room for it beside the list. Its buttons do not
// implement WAI-ARIA tabs (arrow keys, tabpanel), so they use aria-pressed
export function Shelf({ views, narrow, more, foot }: ShelfProps) {
  const [open, setOpen] = useState(false);
  const place = moreIn(open);
  useLayoutEffect(() => {
    if (!open) return;
    const box = document.getElementById('filter-box');
    const rows = [...(box?.querySelectorAll<HTMLButtonElement>('#shelf-more button') ?? [])];
    focusOn(
      firstChip(
        rows,
        (b) => b.offsetParent !== null,
        (b) => b.closest('#shelf-add') !== null,
      ) ??
        box?.querySelector('[data-close]') ??
        null,
    );
  }, [open]);
  return (
    <>
      <nav
        id="shelf"
        aria-label={t('ui_shelf')}
        className="flex min-h-0 flex-col border-r border-line wide:col-span-full wide:block wide:border-r-0 wide:border-b"
      >
        {foot}
        <div
          id="shelf-body"
          className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)] content-start gap-[1.1rem] overflow-y-auto px-2.5 pt-3 pb-4 wide:block wide:overflow-visible wide:px-3 wide:py-[0.6rem]"
        >
          <div
            data-testid="views"
            // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- a fieldset would draw a box and a legend
            role="group"
            aria-label={t('ui_shelf')}
            className="grid grid-cols-[minmax(0,1fr)] gap-1 wide:relative wide:-m-1 wide:flex wide:flex-wrap wide:items-center wide:gap-[5px] wide:p-1"
          >
            {views}
            <NarrowFilter
              {...narrow}
              onOpen={() => {
                setOpen(true);
              }}
            />
          </div>
          {place === 'shelf' ? <ShelfMore inModal={false}>{more}</ShelfMore> : null}
        </div>
      </nav>
      <Dialog
        id="filter-dialog"
        bodyId="filter-box"
        open={open}
        onClosed={() => {
          setOpen(false);
        }}
        afterClose={() => focusOn(document.getElementById('narrow-filter'))}
        labelledBy="filter-title"
        body="fill"
      >
        <DialogHead icon={<FilterIcon />} titleId="filter-title" title={t('ui_narrow')} />
        {place === 'dialog' ? <ShelfMore inModal>{more}</ShelfMore> : null}
        <DialogActions>
          <DialogClose />
        </DialogActions>
      </Dialog>
    </>
  );
}
