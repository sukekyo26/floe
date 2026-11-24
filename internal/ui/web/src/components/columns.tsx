import { useLayoutEffect, useRef, type ReactNode } from 'react';
import { drawThenFocus, focusOn } from '@/components/focus-places';
import { Grip } from '@/components/grip';
import { IconButton } from '@/components/icon-button';
import { ListColumnIcon, ShelfColumnIcon } from '@/components/icons';
import { useColumns, type ColumnState } from '@/hooks/use-columns';
import { t } from '@/lib/t';
import { columns as pageColumns, type ColumnName, type ColumnsStore } from '@/stores/columns';
import { columnWidths } from '@/utils/column-widths';

const CONTROLS: Record<ColumnName, string> = { shelf: 'shelf', list: 'listcol' };

// showColumn opens or folds the column named, as one of its buttons is pressed: drawn at once, the focus goes to the
// button that undoes it
export function showColumn(name: ColumnName, open: boolean, columns: ColumnsStore = pageColumns): void {
  drawThenFocus(
    () => {
      columns.show(name, open);
    },
    `${name}-${open ? 'hide' : 'show'}`,
  );
}

// SHOW_BOTTOM stands what opens a folded column where its button stood, in the middle of the foot's height
export const SHOW_BOTTOM = 'bottom-[calc((var(--spacing-foot)-1px-var(--spacing-icon-btn))/2)]';

// ColumnFoot is the box at a column's foot, under what scrolls, with the button that folds the column; gone under
// 70rem, where the list's columns do not fold, but always there for a column that folds at any width it shows at (the
// frame's tree, the diff's contents). next leaves room on its left for what opens the column before it, standing there
export function ColumnFoot({
  children,
  next = false,
  always = false,
  hidden = false,
}: {
  readonly children?: ReactNode;
  readonly next?: boolean;
  readonly always?: boolean;
  readonly hidden?: boolean;
}) {
  return (
    <div
      data-testid="col-foot"
      hidden={hidden}
      className={`order-1 flex h-foot flex-none items-center border-t border-line pe-foot-pad ${always ? '' : 'wide:hidden '}${next ? 'ps-foot-next' : 'ps-foot-pad'}`}
    >
      {children}
    </div>
  );
}

// HEAD_TITLE is the title in a column's head: the frame's tree's and the diff's contents'
export const HEAD_TITLE =
  'min-w-0 flex-1 truncate font-mono text-label leading-[1.2] font-bold tracking-[0.06em] text-muted uppercase';

// ShelfFoot is the shelf's foot, its button that folds it, for Shelf's foot
export function ShelfFoot({ columns = pageColumns }: { readonly columns?: ColumnsStore }) {
  return (
    <ColumnFoot>
      <IconButton
        id="shelf-hide"
        label={t('ui_shelf_hide')}
        expanded
        controls="shelf"
        onClick={() => {
          showColumn('shelf', false, columns);
        }}
      >
        <ShelfColumnIcon />
      </IconButton>
    </ColumnFoot>
  );
}

interface ShowButtonProps {
  readonly name: ColumnName;
  readonly hidden: boolean;
  readonly raised: boolean;
  // left places it where the button that folded the column stood
  readonly left: string;
  readonly columns: ColumnsStore;
}

// ShowButton opens a column folded, where its button stood, on a ground of its own over whatever lies under it. Drawn
// shown with the focus left in the column it hid (a row picked folds the list, or backToRow does), it takes the focus;
// a press moves it in showColumn too, to this same button (the shelf folds by its button alone, so there that is all).
// LIMIT: kept folded under 70rem, the column shows; the focus left in it falls to the body as the window widens, as
// this button's hidden does not change. Take it on the width too if that comes to trouble someone
function ShowButton({ name, hidden, raised, left, columns }: ShowButtonProps) {
  const button = useRef<HTMLButtonElement>(null);
  useLayoutEffect(() => {
    if (hidden) return;
    if (document.getElementById(CONTROLS[name])?.contains(document.activeElement)) focusOn(button.current);
  }, [hidden, name]);
  return (
    <div className={`fixed ${SHOW_BOTTOM} z-2 flex wide:hidden`} style={{ left }}>
      <IconButton
        ref={button}
        id={`${name}-show`}
        label={t(name === 'shelf' ? 'ui_shelf_show' : 'ui_list_show')}
        hidden={hidden}
        raised={raised}
        display="screen"
        expanded={false}
        controls={CONTROLS[name]}
        onClick={() => {
          showColumn(name, true, columns);
        }}
      >
        {name === 'shelf' ? <ShelfColumnIcon /> : <ListColumnIcon />}
      </IconButton>
    </div>
  );
}

// The grid's columns and rows by screen and what shows: the folded ones from 70rem, a row of the shelf over the list
// under it, one column under 56rem
function gridOf({ screen, shelfOpen, listShown }: ColumnState): string {
  if (screen === 'taskPage')
    return `grid-rows-[minmax(0,1fr)] wide:not-mid:grid-cols-[minmax(0,22rem)_minmax(0,1fr)] mid:grid-cols-[minmax(0,1fr)] ${listShown ? 'not-wide:grid-cols-[var(--list-width)_minmax(0,1fr)]' : 'not-wide:grid-cols-[minmax(0,1fr)]'}`;
  const rows = 'grid-rows-[minmax(0,1fr)] wide:grid-rows-[auto_minmax(0,1fr)]';
  if (screen === 'taskList')
    return `${rows} wide:grid-cols-[minmax(0,1fr)] ${shelfOpen ? 'not-wide:grid-cols-[var(--shelf-width)_minmax(0,1fr)]' : 'not-wide:grid-cols-[minmax(0,1fr)]'}`;
  const wide =
    shelfOpen && listShown
      ? 'not-wide:grid-cols-[var(--shelf-width)_var(--list-width)_minmax(0,1fr)]'
      : shelfOpen
        ? 'not-wide:grid-cols-[var(--shelf-width)_minmax(0,1fr)]'
        : listShown
          ? 'not-wide:grid-cols-[var(--list-width)_minmax(0,1fr)]'
          : 'not-wide:grid-cols-[minmax(0,1fr)]';
  return `${rows} ${wide} wide:not-mid:grid-cols-[22rem_minmax(0,1fr)] mid:grid-cols-[minmax(0,1fr)]`;
}

export interface ColumnsProps {
  // shelf is the screen's Shelf (nav#shelf, its foot ShelfFoot), left out on a task's page; list what the list's
  // column holds under its foot; reader the page beside the list (ReaderPane), left out on the task list
  readonly shelf?: ReactNode;
  readonly list: ReactNode;
  readonly reader?: ReactNode;
  // alone lays out the list's column alone, the rest drawn but hidden, as tests read them there
  readonly alone?: boolean;
  readonly columns?: ColumnsStore;
}

// Columns lays out the screen of a list: the shelf, the list and the page beside it, the edges between them and the
// buttons that fold and open the two columns, by the screen, what is kept and the window. The widths go to the root
// (--shelf-width, --list-width), as the edges stand fixed on the window
export function Columns({ shelf, list, reader, alone = false, columns = pageColumns }: ColumnsProps) {
  const state = useColumns(columns);
  const { screen, kept, at, widths, shelfOpen, listShown, picked } = state;
  const hasShelf = screen !== 'taskPage';
  const taskList = screen === 'taskList';
  useLayoutEffect(() => {
    const root = document.documentElement.style;
    root.setProperty('--shelf-width', `${widths.shelf}px`);
    root.setProperty('--list-width', `${widths.list}px`);
  }, [widths.shelf, widths.list]);
  // A width asked for is kept within the bounds of now
  const setWidth = (name: ColumnName, width: number) => {
    columns.set({ [name]: columnWidths({ ...kept, [name]: width }, at)[name] });
  };
  const listLeft = () => document.getElementById('listcol')?.getBoundingClientRect().left ?? 0;
  return (
    <main
      id="manage"
      className={`grid min-h-0 flex-1 ${alone ? 'grid-cols-[minmax(0,1fr)] grid-rows-[minmax(0,1fr)] [&>#shelf]:hidden' : `${gridOf(state)} ${hasShelf && !shelfOpen ? 'not-wide:[&>#shelf]:hidden' : ''}`}`}
    >
      <h1 className="sr-only">floe</h1>
      {hasShelf ? (
        <ShowButton
          name="shelf"
          hidden={shelfOpen || alone}
          raised={!listShown}
          left="var(--spacing-foot-pad)"
          columns={columns}
        />
      ) : null}
      {taskList ? null : (
        <ShowButton
          name="list"
          hidden={listShown || alone}
          raised
          left={
            !hasShelf
              ? 'var(--spacing-foot-pad)'
              : shelfOpen
                ? 'calc(var(--shelf-width) + var(--spacing-foot-pad))'
                : 'var(--spacing-foot-next)'
          }
          columns={columns}
        />
      )}
      {hasShelf ? shelf : null}
      {hasShelf ? (
        <div className="contents wide:hidden">
          <Grip
            id="shelf-grip"
            label={t('ui_shelf_width')}
            controls="shelf"
            at={widths.shelf}
            width={widths.shelf}
            min={widths.shelfMin}
            max={widths.shelfMax}
            hidden={!shelfOpen || alone}
            onSet={(w) => {
              setWidth('shelf', w);
            }}
            onKeep={columns.keep}
          />
        </div>
      ) : null}
      <section
        id="listcol"
        className={`flex min-h-0 min-w-0 flex-col bg-panel mid:border-0 ${taskList ? '' : 'border-r border-line'} ${listShown || alone ? '' : 'not-wide:hidden'}`}
      >
        {taskList && shelfOpen ? null : (
          <ColumnFoot next={hasShelf && !shelfOpen} hidden={alone}>
            {taskList ? null : (
              <IconButton
                id="list-hide"
                label={t('ui_list_hide')}
                expanded
                controls="listcol"
                hidden={screen === 'taskPage' && !picked}
                onClick={() => {
                  showColumn('list', false, columns);
                }}
              >
                <ListColumnIcon />
              </IconButton>
            )}
          </ColumnFoot>
        )}
        <div id="list-body" className="min-h-0 flex-1 overflow-y-auto bg-bg">
          {list}
        </div>
      </section>
      {taskList ? null : (
        <div className="contents wide:hidden">
          <Grip
            id="list-grip"
            label={t('ui_list_width')}
            controls="listcol"
            at={widths.shelf + widths.list}
            width={widths.list}
            min={widths.listMin}
            max={widths.listMax}
            left={listLeft}
            hidden={!listShown || alone}
            onSet={(w) => {
              setWidth('list', w);
            }}
            onKeep={columns.keep}
          />
        </div>
      )}
      {/* The task list's is there too, hidden (the tests read it there) */}
      <div className="contents" hidden={taskList || alone}>
        {reader}
      </div>
    </main>
  );
}
