import { useCallback, useMemo } from 'react';

import { ArchivedBadge } from '@/components/archived-badge';
import { RowActions } from '@/components/page-actions';
import type { Openers } from '@/components/page-menu';
import { PageRow } from '@/components/page-row';
import { RowSub } from '@/components/row-sub';
import { Rows, type RowItem } from '@/components/rows';
import { TagPill } from '@/components/tag-pill';
import { TreeRow } from '@/components/tree-row';
import { groupName } from '@/features/tasks/utils/add-page';
import { taskGroups } from '@/features/tasks/utils/task-page';
import { pageTitlePress, useRowPress } from '@/hooks/use-row-press';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { folders as pageFolders, type Folders } from '@/stores/open-folders';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pages as pagePages, type Pages } from '@/stores/pages';
import type { TaskOf } from '@/types/task';
import { unseen } from '@/utils/list-view';
import { treeItems } from '@/utils/tree-items';

export interface TaskPageRowsProps {
  readonly task: TaskOf | null;
  readonly openers: Openers;
  readonly pages?: Pages;
  readonly folders?: Folders;
  readonly openPage?: OpenPage;
}

// No projects' colours on a task's page: its pages are all of its project
const NO_RANKS: ReadonlyMap<string, number> = new Map();
const nothingSelected = () => false;

// TaskPageRows is the rows of a task's page: under no category where it has pages, then its categories in their order,
// one of no page saying so. As the page list's rows but for what a task's page leaves out: no
// box nor "Select", no way to its own task or to a tag's list, the project's pill; an archived page stays, dimmed and
// badged
export function TaskPageRows({
  task,
  openers,
  pages = pagePages,
  folders = pageFolders,
  openPage = pageOpen,
}: TaskPageRowsProps) {
  const { opened, listings } = useStore(folders.list);
  const picked = useStore(openPage.picked);
  const groups = useMemo(() => (task ? taskGroups(task) : []), [task]);
  const drawn = useCallback(() => groups.flatMap((g) => g.entries), [groups]);
  const presses = useRowPress({ selecting: nothingSelected, drawn, openPage });
  const onFolder = useCallback(
    (slug: string) => {
      void folders.toggle(slug, '');
    },
    [folders],
  );
  const isPicked = (slug: string, file: string) => picked?.slug === slug && picked.file === file;
  const shownTask = task ? { project: task.project, name: task.name } : null;
  const rows: RowItem[] = [];
  let index = 0;
  for (const g of groups) {
    const { category } = g;
    if (task)
      rows.push({
        kind: 'group',
        key: `group:${category}`,
        heading: groupName(category),
        ...(g.entries.length === 0 ? { empty: t('ui_category_empty') } : {}),
      });
    for (const a of g.entries) {
      const at = index++;
      rows.push({
        kind: 'row',
        key: `entry:${a.slug}`,
        titled: true,
        render: (kept) => (
          <PageRow
            page={a}
            index={Math.min(at, 30)}
            picked={isPicked(a.slug, '')}
            unread={unseen(a)}
            archived={Boolean(a.archived)}
            kept={kept}
            folderOpen={opened.has(a.slug)}
            sub={
              <RowSub
                page={a}
                ranks={NO_RANKS}
                middle={
                  <>
                    {a.archived ? <ArchivedBadge /> : null}
                    {a.tags?.length ? (
                      <span data-testid="tags" className="relative z-1 flex flex-wrap gap-[0.3rem]">
                        {a.tags.map((name) => (
                          <TagPill key={name} name={name} />
                        ))}
                      </span>
                    ) : null}
                  </>
                }
              />
            }
            actions={<RowActions page={a} openers={openers} taskShown={shownTask} />}
            onTitleClick={(e) => {
              pageTitlePress(a, presses, onFolder, e);
            }}
            onTitleKeyDown={(e) => {
              presses.key({ slug: a.slug, file: '' }, e);
            }}
          />
        ),
      });
      if (!a.folder || a.missing || !opened.has(a.slug)) continue;
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
              archived={Boolean(a.archived)}
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
  return <Rows items={rows} selecting={false} pages={pages} />;
}
