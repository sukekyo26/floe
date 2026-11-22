import { useEffect } from 'react';

import type { Openers } from '@/components/page-menu';
import { Rows, type RowItem } from '@/components/rows';
import { TreeRow } from '@/components/tree-row';
import { getBoot } from '@/config/boot';
import { ListEmpty } from '@/features/page-list/components/list-empty';
import { ListRow } from '@/features/page-list/components/list-row';
import { dayGroups } from '@/features/page-list/utils/list-days';
import { emptyOf } from '@/features/page-list/utils/list-empty';
import { NO_PICKS, tagPress, unseenSlugs } from '@/features/page-list/utils/list-shelf';
import { useMinute } from '@/hooks/use-minute';
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
}

// PageList is the page list's rows (ui.html renderList): the entries the shelf and the search leave, newest first under
// their days' headings, a folder open followed by its tree; or, with none to draw, the words why. The view "new" keeps
// every unread page as it draws, so that one read stays on it
export function PageList({
  openers,
  onAdd,
  query = listQuery,
  pages = pagePages,
  folders = pageFolders,
  openPage = pageOpen,
}: PageListProps) {
  const state = useStore(query.query);
  const items = useStore(pages.state).entries;
  const ended = useStore(pages.ended);
  const { opened, listings } = useStore(folders.list);
  const picked = useStore(openPage.picked);
  const now = useMinute();
  const { seen } = state;

  useEffect(() => {
    if (state.view === 'new') query.see(unseenSlugs(items));
  }, [query, state.view, items]);

  const shown = arrange(items, state, seen);
  const ranks = projectOrder(items);
  const isPicked = (slug: string, file: string) => picked?.slug === slug && picked.file === file;
  const toggle = (slug: string, dir: string) => {
    void folders.toggle(slug, dir);
  };
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
            index={at}
            ranks={ranks}
            openers={openers}
            picked={isPicked(a.slug, '')}
            unread={unseen(a)}
            kept={kept}
            folderOpen={opened.has(a.slug)}
            onTag={(name) => {
              const change = tagPress(state, name);
              if (change) query.filter(change);
            }}
            onFolder={(slug) => {
              toggle(slug, '');
            }}
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
              onToggle={(path) => {
                toggle(a.slug, path);
              }}
            />
          ),
        });
    }
  }

  return (
    <>
      <Rows items={rows} selecting={false} pages={pages} />
      <ListEmpty
        shown={emptyOf(items, state, seen, shown, ended)}
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
