import { useEffect, useRef } from 'react';

import { ColumnFoot, HEAD_TITLE, SHOW_BOTTOM } from '@/components/columns';
import { drawThenFocus } from '@/components/focus-places';
import { Grip } from '@/components/grip';
import { IconButton } from '@/components/icon-button';
import { SideIcon } from '@/components/icons';
import { diffToc as sharedToc } from '@/features/versions/stores/diff-toc';
import { levelOf, sectionName, type Section } from '@/features/versions/utils/diff-words';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import type { KeptSide } from '@/stores/kept-side';
import { clampSide, SIDE_MIN, sideMax } from '@/utils/edge';

const LEVEL: Record<number, string> = { 1: '', 2: '[&>*]:ps-[1.2rem]', 3: '[&>*]:ps-[1.9rem]' };
const MARK: Record<
  string,
  {
    tone: string;
    mark: 'ui_diff_mark_changed' | 'ui_diff_mark_added' | 'ui_diff_mark_removed';
    said: 'ui_diff_said_changed' | 'ui_diff_said_added' | 'ui_diff_said_removed';
  }
> = {
  changed: { tone: 'bg-chg-soft text-chg', mark: 'ui_diff_mark_changed', said: 'ui_diff_said_changed' },
  added: { tone: 'bg-ins-soft text-ins', mark: 'ui_diff_mark_added', said: 'ui_diff_said_added' },
  removed: { tone: 'bg-del-soft text-del', mark: 'ui_diff_mark_removed', said: 'ui_diff_said_removed' },
};

// Mark is a section's state, a letter seen and its word read aloud
function Mark({ state }: { readonly state: string }) {
  const m = MARK[state];
  if (!m) return null;
  return (
    <span
      className={`ms-auto inline-block flex-none rounded-full px-[0.35rem] text-label leading-[1.5] font-bold ${m.tone}`}
    >
      <span aria-hidden="true">{t(m.mark)}</span>
      <span className="sr-only">{`${t(m.said)} `}</span>
    </span>
  );
}

interface DiffTocProps {
  readonly sections: readonly Section[];
  // section is the one n, p or a press went to last, -1 for none
  readonly section: number;
  readonly onPlace: (i: number) => void;
  // width is the contents' width kept within bounds (clampSide), folded too; innerWidth the window's, for those bounds
  readonly width: number;
  readonly innerWidth: number;
  readonly toc?: KeptSide;
}

const ITEM =
  'flex min-h-7 w-full items-center gap-1.5 rounded-small px-2 py-0.5 [&>span:first-child]:min-w-0 [&>span:first-child]:truncate';

// DiffToc is the contents of the later version, each section that has a place to go to a button, its width dragged and
// kept, folded and opened. Its own width is kept apart from the frame's tree
export function DiffToc({ sections, section, onPlace, width, innerWidth, toc = sharedToc }: DiffTocProps) {
  const side = useStore(toc.side());
  const nav = useRef<HTMLElement>(null);
  const list = useRef<HTMLDivElement>(null);
  useEffect(() => {
    list.current?.querySelector('[data-on]')?.scrollIntoView({ block: 'nearest' });
  }, [section]);
  // The focus goes to the button that undoes it, as it is pressed: not as it is drawn first
  const setOpen = (open: boolean) => {
    drawThenFocus(
      () => {
        toc.setOpen(open);
        toc.keep();
      },
      open ? 'diff-toc-hide' : 'diff-toc-show',
    );
  };
  return (
    <>
      <nav
        ref={nav}
        id="diff-toc-nav"
        aria-labelledby="diff-toc-title"
        hidden={!side.open}
        className="flex min-h-0 flex-col text-small mid:hidden"
      >
        <ColumnFoot always>
          <IconButton
            id="diff-toc-hide"
            label={t('ui_diff_toc_hide')}
            expanded
            controls="diff-toc-nav"
            onClick={() => {
              setOpen(false);
            }}
          >
            <SideIcon />
          </IconButton>
        </ColumnFoot>
        <div className="flex min-h-[2.6rem] flex-none items-center px-4 pt-[0.55rem] pb-[0.45rem]">
          <h2 id="diff-toc-title" className={`m-0 ${HEAD_TITLE}`}>
            {t('ui_diff_toc')}
          </h2>
        </div>
        <div ref={list} id="diff-toc" className="relative min-h-0 flex-1 overflow-auto px-2">
          {sections.map((s, i) => {
            const name = <span>{sectionName(s)}</span>;
            const mark = s.state === 'same' ? null : <Mark state={s.state} />;
            return (
              // oxlint-disable-next-line react/no-array-index-key -- a section is its place in the answer, drawn anew with it
              <div key={i} className={LEVEL[levelOf(s)]}>
                {s.place === undefined ? (
                  <div className={`${ITEM} text-muted`}>
                    {name}
                    {mark}
                  </div>
                ) : (
                  <button
                    type="button"
                    data-section={i}
                    data-on={i === section ? 'true' : undefined}
                    onClick={() => {
                      onPlace(i);
                    }}
                    className={`${ITEM} cursor-pointer border-0 bg-transparent text-start font-[650] text-fg data-on:bg-sel`}
                  >
                    {name}
                    {mark}
                  </button>
                )}
              </div>
            );
          })}
        </div>
        <p className="m-0 min-h-0 flex-[0_1_auto] overflow-hidden border-t border-line px-4 py-2.5 text-label leading-[1.9] text-muted [&_kbd]:me-[0.3rem] [&_kbd]:rounded-small [&_kbd]:border [&_kbd]:border-line [&_kbd]:bg-subtle [&_kbd]:px-[0.3rem] [&_kbd]:font-[inherit]">
          <kbd>n</kbd>
          {t('ui_diff_next')}
          <br />
          <kbd>p</kbd>
          {t('ui_diff_prev')}
          <br />
          <kbd>d</kbd>
          {t('ui_diff_close')}
        </p>
      </nav>
      <Grip
        id="diff-toc-grip"
        label={t('ui_diff_toc_width')}
        controls="diff-toc-nav"
        place="box"
        // The box's left, which the contents start at: the frame's is the window's
        left={() => nav.current?.getBoundingClientRect().left ?? 0}
        at={width}
        width={width}
        min={SIDE_MIN}
        max={sideMax(innerWidth)}
        hidden={!side.open}
        onSet={(next) => {
          toc.setWidth(clampSide(next, innerWidth));
        }}
        onKeep={toc.keep}
      />
      {/* Where the button that folded it stood, on a ground of its own: placed by its box, gone with the contents
          under 56rem */}
      <div hidden={side.open} className={`absolute ${SHOW_BOTTOM} left-foot-pad z-2 flex mid:hidden`}>
        <IconButton
          id="diff-toc-show"
          label={t('ui_diff_toc_show')}
          hidden={side.open}
          raised
          display="screen"
          expanded={false}
          controls="diff-toc-nav"
          onClick={() => {
            setOpen(true);
          }}
        >
          <SideIcon />
        </IconButton>
      </div>
    </>
  );
}
