import { useCallback, useEffect, useMemo } from 'react';

import { backToRow } from '@/components/focus-places';
import type { Openers } from '@/components/page-menu';
import { Rows, type RowItem } from '@/components/rows';
import { TreeRow } from '@/components/tree-row';
import { getBoot } from '@/config/boot';
import { ListEmpty } from '@/features/page-list/components/list-empty';
import { ListRow } from '@/features/page-list/components/list-row';
import { dayGroups } from '@/features/page-list/utils/list-days';
import { emptyOf } from '@/features/page-list/utils/list-empty';
import { keepNewSeen } from '@/features/page-list/stores/keep-new-seen';
import { followList, selection as pageSelection, type Selection } from '@/features/page-list/stores/selection';
import { NO_PICKS, tagPress } from '@/features/page-list/utils/list-shelf';
import { useBackToRow } from '@/hooks/use-back-to-row';
import { useMinute } from '@/hooks/use-minute';
import { useOneColumn } from '@/hooks/use-one-column';
import { useRowPress } from '@/hooks/use-row-press';
import { useStore } from '@/hooks/use-store';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { folders as pageFolders, type Folders } from '@/stores/open-folders';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { arrange, unseen } from '@/utils/list-view';
import { projectOrder } from '@/utils/project-order';
import { treeItems } from '@/utils/tree-items';

export interface PageListProps {
  readonly openers: Openers;
  // onAdd opens the modal that adds a link (P5d's)
  readonly onAdd: () => void;
  readonly query?: ListQuery;
  readonly pages?: Pages;
  readonly folders?: Folders;
  readonly openPage?: OpenPage;
  readonly selection?: Selection;
}

// PageList is the page list's rows: the entries the shelf and the search leave, newest first under their days'
// headings, a folder open followed by its tree; or, with none to draw, the words why. The view "new" keeps every unread
// page as it draws, so that one read stays on it
export function PageList({
  openers,
  onAdd,
  query = listQuery,
  pages = pagePages,
  folders = pageFolders,
  openPage = pageOpen,
  selection = pageSelection,
}: PageListProps) {
  const state = useStore(query.query);
  const { read, entries: items } = useStore(pages.state);
  const { opened, listings } = useStore(folders.list);
  const picked = useStore(openPage.picked);
  const selected = useStore(selection.selected);
  const now = useMinute();
  const { seen } = state;

  useEffect(() => keepNewSeen(query, pages), [query, pages]);
  useEffect(() => followList(query, pages, selection), [query, pages, selection]);

  const shown = arrange(items, state, seen);
  // The same values and functions for every row through redraws, so that a row drawn once is not drawn again (memo)
  const ranks = useMemo(() => projectOrder(items), [items]);
  const onTag = useCallback(
    (name: string) => {
      const change = tagPress(query.query.get(), name);
      if (change) query.filter(change);
    },
    [query],
  );
  const onFolder = useCallback(
    (slug: string) => {
      void folders.toggle(slug, '');
    },
    [folders],
  );
  const drawn = useCallback(() => {
    const now = query.query.get();
    return arrange(pages.state.get().entries, now, now.seen);
  }, [query, pages]);
  const selecting = useCallback(() => selection.selected.get().size > 0, [selection]);
  const presses = useRowPress({ selecting, drawn, openPage });
  const onPick = useCallback(
    (slug: string) => {
      selection.toggle(slug, true);
    },
    [selection],
  );
  const onRemoved = useCallback(
    (slug: string) => {
      selection.toggle(slug, false);
    },
    [selection],
  );
  useOneColumn(openPage);
  const back = useCallback(() => {
    backToRow({ pages, openPage });
  }, [pages, openPage]);
  useBackToRow(read, back);
  const isPicked = (slug: string, file: string) => picked?.slug === slug && picked.file === file;
  const rows: RowItem[] = [];
  let index = 0;
  for (const day of dayGroups(shown, new Date(now), getBoot().lang)) {
    rows.push({ kind: 'group', key: day.key, heading: day.heading });
    for (const a of day.entries) {
      const at = index++;
      const open = Boolean(a.folder) && !a.missing && opened.has(a.slug);
      rows.push({
        kind: 'row',
        key: `entry:${a.slug}`,
        titled: true,
        render: (kept) => (
          <ListRow
            page={a}
            // Past 30 the rise waits no longer (PageRow): a row added above leaves the rows below it as they were
            index={Math.min(at, 30)}
            ranks={ranks}
            openers={openers}
            picked={isPicked(a.slug, '')}
            unread={unseen(a)}
            kept={kept}
            folderOpen={opened.has(a.slug)}
            onTag={onTag}
            onFolder={onFolder}
            presses={presses}
            selected={selected.has(a.slug)}
            onSelect={selection.toggle}
            onPick={onPick}
            onRemoved={onRemoved}
          />
        ),
      });
      if (!open) continue;
      const folder = { slug: a.slug, path: a.path ?? '' };
      for (const item of treeItems(a.slug, '', 1, listings, opened))
        rows.push({
          kind: 'row',
          key: item.key,
          titled: item.kind !== 'note',
          render: () => (
            <TreeRow
              item={item}
              folder={folder}
              picked={item.kind === 'page' && isPicked(a.slug, item.file)}
              onTitleClick={(e) => {
                if (item.kind === 'page') presses.press({ slug: a.slug, file: item.file }, e);
              }}
              onTitleKeyDown={(e) => {
                if (item.kind === 'page') presses.key({ slug: a.slug, file: item.file }, e);
              }}
              onToggle={(path) => {
                void folders.toggle(a.slug, path);
              }}
            />
          ),
        });
    }
  }

  return (
    <>
      <Rows items={rows} selecting={selected.size > 0} pages={pages} />
      <ListEmpty
        shown={emptyOf(items, state, seen, shown, read)}
        archived={items.filter((a) => a.archived).length}
        onAdd={onAdd}
        onShowArchived={() => {
          query.filter({ view: 'archived' });
        }}
        onClear={() => {
          query.setWords('');
          query.filter(NO_PICKS);
        }}
      />
    </>
  );
}
