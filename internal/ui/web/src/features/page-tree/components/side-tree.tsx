import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import { ColumnFoot, HEAD_TITLE, SHOW_BOTTOM } from '@/components/columns';
import { drawThenFocus } from '@/components/focus-places';
import { Grip } from '@/components/grip';
import { IconButton } from '@/components/icon-button';
import { SideIcon } from '@/components/icons';
import { SideRow } from '@/features/page-tree/components/side-row';
import { frameSide } from '@/features/page-tree/stores/side';
import { sideItems } from '@/features/page-tree/utils/side-items';
import { sideLayout } from '@/features/page-tree/utils/side-layout';
import { useMedia } from '@/hooks/use-media';
import { useStore } from '@/hooks/use-store';
import { useWindowWidth } from '@/hooks/use-window-width';
import { NARROW } from '@/lib/platform';
import { t } from '@/lib/t';
import type { KeptSide } from '@/stores/kept-side';
import { folders as pageFolders, type Folders } from '@/stores/open-folders';
import type { Page } from '@/types/page';
import { clampSide, SIDE_MIN, sideMax } from '@/utils/edge';

export interface SideTreeProps {
  // folder is the folder framed, as the list of pages has it; null while it is not found, which hides the tree
  readonly folder: Page | null;
  // file is the folder's page framed (?file=)
  readonly file: string;
  readonly folders?: Folders;
  readonly side?: KeptSide;
}

// The page open now, in the middle of the tree's sight
const toCurrent = () => {
  document.querySelector('#side-tree [aria-current="page"]')?.scrollIntoView({ block: 'center' });
};

// SideTree is the tree on the frame's left of a folder's page, its edge and what opens it folded: beside the page, its
// width and whether it shows kept in the browser; in a narrow window over the page when asked, its edge gone, keeping
// nothing. The width it takes of the page goes to the root (--side), none while it is hidden, over the page or gone
export function SideTree({ folder, file, folders = pageFolders, side = frameSide }: SideTreeProps) {
  const kept = useStore(side.side());
  const { opened, listings } = useStore(folders.side);
  const narrow = useMedia(NARROW);
  const innerWidth = useWindowWidth();
  // Over the page in a narrow window, as asked there alone: crossing the width closes it
  const [over, setOver] = useState(false);
  useEffect(() => {
    const media = window.matchMedia(NARROW);
    const change = () => {
      setOver(false);
    };
    media.addEventListener('change', change);
    return () => {
      media.removeEventListener('change', change);
    };
  }, []);
  const laid = sideLayout({ open: kept.open, over, narrow, width: kept.width, innerWidth });
  const found = folder !== null;
  const shown = found && laid.shown;
  const items = folder === null ? [] : sideItems(folder.slug, opened, listings);
  useLayoutEffect(() => {
    const root = document.documentElement.style;
    root.setProperty('--side-width', `${laid.width}px`);
    root.setProperty('--side', `${found ? laid.take : 0}px`);
  }, [found, laid.width, laid.take]);
  useLayoutEffect(
    () => () => {
      document.documentElement.style.setProperty('--side', '0px');
    },
    [],
  );
  // The page open now is brought into sight the first time the tree shows rows, not at each redraw
  const brought = useRef(false);
  useEffect(() => {
    if (brought.current || !shown || items.length === 0) return;
    brought.current = true;
    toCurrent();
  }, [shown, items.length]);
  const show = (open: boolean) => {
    drawThenFocus(
      () => {
        if (narrow) setOver(open);
        else {
          side.setOpen(open);
          side.keep();
        }
      },
      open ? 'side-hide' : 'side-show',
    );
    if (open) toCurrent();
  };
  return (
    <>
      <nav
        id="side"
        aria-label={folder?.title}
        hidden={!shown}
        data-over={narrow ? 'true' : undefined}
        className={`fixed top-bar bottom-0 left-0 z-2 flex flex-col border-r border-line bg-panel print:hidden [&_:is(a,button):focus-visible]:-outline-offset-2 ${narrow ? 'w-[min(80vw,var(--side-width))] shadow-sm ring-1 ring-line' : 'w-(--side-width)'}`}
      >
        <ColumnFoot always>
          <IconButton
            id="side-hide"
            label={t('ui_side_hide')}
            expanded
            controls="side"
            onClick={() => {
              show(false);
            }}
          >
            <SideIcon />
          </IconButton>
        </ColumnFoot>
        <div className="flex min-h-[2.6rem] flex-none items-center px-[0.8rem] pt-[0.55rem] pb-[0.45rem]">
          <span id="side-title" className={HEAD_TITLE}>
            {folder?.title}
          </span>
        </div>
        <div id="side-tree" className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-[0.3rem] pb-[0.8rem]">
          {folder === null
            ? null
            : items.map((item) => (
                <SideRow
                  key={item.key}
                  item={item}
                  slug={folder.slug}
                  file={file}
                  onToggle={(dir) => {
                    void folders.toggleSide(folder.slug, file, dir);
                  }}
                />
              ))}
        </div>
      </nav>
      <Grip
        id="side-grip"
        label={t('ui_side_width')}
        controls="side"
        at={laid.width}
        width={laid.width}
        min={SIDE_MIN}
        max={sideMax(innerWidth)}
        hidden={!shown || narrow}
        onSet={(w) => {
          side.setWidth(clampSide(w, innerWidth));
        }}
        onKeep={side.keep}
      />
      <div className={`fixed ${SHOW_BOTTOM} left-foot-pad z-2 flex`}>
        <IconButton
          id="side-show"
          label={t('ui_side_show')}
          hidden={folder === null || shown}
          raised
          display="screen"
          expanded={false}
          controls="side"
          onClick={() => {
            show(true);
          }}
        >
          <SideIcon />
        </IconButton>
      </div>
    </>
  );
}
