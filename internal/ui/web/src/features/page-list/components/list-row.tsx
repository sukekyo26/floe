import { memo } from 'react';

import { RowActions } from '@/components/page-actions';
import type { Openers } from '@/components/page-menu';
import { PageRow } from '@/components/page-row';
import { RowSub } from '@/components/row-sub';
import { TagPill } from '@/components/tag-pill';
import { pageTitlePress, type RowPresses } from '@/hooks/use-row-press';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';
import { TASKS_PATH, taskHref } from '@/utils/page-urls';

export interface ListRowProps {
  readonly page: Page;
  readonly index: number;
  readonly ranks: ReadonlyMap<string, number>;
  readonly openers: Openers;
  readonly picked: boolean;
  readonly unread: boolean;
  readonly kept: boolean;
  readonly folderOpen: boolean;
  // onTag narrows the list to a tag of the row; onFolder opens or closes a folder's tree
  readonly onTag: (name: string) => void;
  readonly onFolder: (slug: string) => void;
  // presses is what the title does as pressed and as Enter lands on it, the same for every row
  readonly presses: RowPresses;
  // The selection's, which the list gives once it has one (P5d): by the page's slug, the same functions for every row
  readonly selected?: boolean;
  readonly onSelect?: (slug: string, on: boolean) => void;
  readonly onPick?: (slug: string) => void;
  readonly onRemoved?: (slug: string) => void;
}

const TASK_TAG =
  'relative z-1 rounded-small border border-dashed border-box-line px-[0.35rem] truncate text-muted no-underline hover:border-faint hover:text-fg';

// ListRow is a row of the page list: the shared row, with the way to the task the page is on or the task list it is
// pinned on, and its tags, each narrowing the list to itself. A folder's title opens its tree. Drawn anew only as its
// props change (memo): a redraw of the list (a row picked, a minute) draws every row otherwise, 400 ms for 1000 rows in
// the gallery's dev build (65 ms with memo)
export const ListRow = memo(function ListRow({
  page: a,
  index,
  ranks,
  openers,
  picked,
  unread,
  kept,
  folderOpen,
  onTag,
  onFolder,
  presses,
  selected = false,
  onSelect,
  onPick,
  onRemoved,
}: ListRowProps) {
  const middle = (
    <>
      {a.task?.name ? (
        <a data-testid="task-tag" href={taskHref({ project: a.taskProject, name: a.task.name })} className={TASK_TAG}>
          {t('ui_task_badge', a.task.name)}
        </a>
      ) : null}
      {a.task?.pin && !a.archived ? (
        <a
          data-testid="task-tag"
          href={`${TASKS_PATH}?${new URLSearchParams({ project: a.taskProject }).toString()}`}
          className={TASK_TAG}
        >
          {t('ui_pin_badge')}
        </a>
      ) : null}
      {a.tags?.length ? (
        <span data-testid="tags" className="relative z-1 flex flex-wrap gap-[0.3rem]">
          {a.tags.map((name) => (
            <TagPill
              key={name}
              name={name}
              onPress={() => {
                onTag(name);
              }}
            />
          ))}
        </span>
      ) : null}
    </>
  );
  return (
    <PageRow
      page={a}
      index={index}
      picked={picked}
      unread={unread}
      kept={kept}
      selected={selected}
      folderOpen={folderOpen}
      sub={<RowSub page={a} ranks={ranks} middle={middle} />}
      actions={
        <RowActions
          page={a}
          openers={openers}
          {...(onPick
            ? {
                onPick: () => {
                  onPick(a.slug);
                },
              }
            : {})}
          {...(onRemoved
            ? {
                onRemoved: () => {
                  onRemoved(a.slug);
                },
              }
            : {})}
        />
      }
      {...(onSelect
        ? {
            onSelect: (on: boolean) => {
              onSelect(a.slug, on);
            },
          }
        : {})}
      onTitleClick={(e) => {
        pageTitlePress(a, presses, onFolder, e);
      }}
      onTitleKeyDown={(e) => {
        presses.key({ slug: a.slug, file: '' }, e);
      }}
    />
  );
});
