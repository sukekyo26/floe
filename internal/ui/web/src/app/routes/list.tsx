import { useEffect, useState } from 'react';

import { Bar } from '@/app/bar';
import { appKeys, pressDiff, pressStar } from '@/app/keys';
import { useScreenModals } from '@/app/modals';
import { PageReader } from '@/app/routes/reader';
import { Columns, ShelfFoot, showColumn } from '@/components/columns';
import { focusOn, moveRow, tickRow } from '@/components/focus-places';
import { ListHeadBox } from '@/components/list-head-box';
import { AddDialog } from '@/features/page-list/components/add-dialog';
import { BulkBar } from '@/features/page-list/components/bulk-bar';
import { ListCount, ListWhat, useWhatHidden } from '@/features/page-list/components/list-head';
import { ListShelf } from '@/features/page-list/components/list-shelf';
import { PageList } from '@/features/page-list/components/page-list';
import { SearchBox } from '@/features/page-list/components/search-box';
import { syncList } from '@/features/page-list/stores/sync-list';
import { useColumns } from '@/hooks/use-columns';
import { useKeys } from '@/hooks/use-keys';
import { useRowKeys } from '@/hooks/use-row-keys';

const showShelf = () => {
  showColumn('shelf', true);
};

// ListRoute is the page list, /: the bar with the list's search, the shelf, the list under its head (what it is
// narrowed to while the shelf is folded, how many, the bar of the selected) and the page beside it, the modals every
// screen opens (useScreenModals) and the one that adds a link. Its state and keys are inside the app's DrawGuard
export function ListRoute() {
  const [adding, setAdding] = useState(false);
  const { openers, openFind, findToggle, openHelp, openSettings, openTags, modals } = useScreenModals('list');
  const { listShown } = useColumns();
  const whatHidden = useWhatHidden();
  useEffect(() => syncList(), []);
  const openAdd = () => {
    setAdding(true);
  };

  const rows = useRowKeys({ move: moveRow, tick: tickRow });
  const keys = appKeys({
    rows,
    find: openFind,
    search: () => {
      focusOn(document.getElementById('q'));
    },
    findToggle,
    help: () => {
      openHelp('keys');
    },
    star: pressStar,
    diff: pressDiff,
  });
  useKeys(keys);

  // What the list is narrowed to stands in the list's head while the list shows, else in the head of the page beside
  // it; hidden, in neither, so that the page's head goes with no page picked
  const what = <ListWhat onShow={showShelf} />;
  return (
    <>
      <Bar
        screen="list"
        search={<SearchBox />}
        onFind={openFind}
        onHelp={() => {
          openHelp('about');
        }}
        onSettings={openSettings}
      />
      {/* After the mistake's words, which stand outside the DrawGuard and so later in the document (ErrorBar) */}
      <div className="order-1 flex min-h-0 flex-1 flex-col">
        <Columns
          shelf={<ListShelf onAdd={openAdd} foot={<ShelfFoot />} />}
          list={
            <>
              <ListHeadBox>
                {listShown ? what : null}
                <ListCount />
                <BulkBar onTags={openTags} />
              </ListHeadBox>
              <PageList openers={openers} onAdd={openAdd} />
            </>
          }
          reader={
            <PageReader
              openers={openers}
              keys={new Set(Object.keys(keys))}
              onKey={(key) => {
                keys[key]?.();
              }}
              what={listShown || whatHidden ? null : what}
            />
          }
        />
      </div>
      {modals}
      <AddDialog
        open={adding}
        onClosed={() => {
          setAdding(false);
        }}
      />
    </>
  );
}
