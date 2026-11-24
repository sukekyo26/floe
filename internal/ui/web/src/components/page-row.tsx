import type { KeyboardEventHandler, MouseEventHandler, ReactNode } from 'react';

import { FolderMark, LinkMark } from '@/components/icons';
import { RowCard } from '@/components/row-card';
import { t } from '@/lib/t';
import { fmt } from '@/lib/time';
import type { Page } from '@/types/page';
import { kindOf } from '@/utils/list-view';
import { pageHref } from '@/utils/page-urls';

export interface PageRowProps {
  readonly page: Page;
  // index is its place in the list, which staggers its rise
  readonly index: number;
  // picked: its page open beside the list; selected: its box ticked; unread: a version not opened yet; archived: dimmed
  // as such (on a task's page); kept: drawn before, so not risen in again
  readonly picked?: boolean;
  readonly selected?: boolean;
  readonly unread?: boolean;
  readonly archived?: boolean;
  readonly kept?: boolean;
  // folderOpen: a folder's tree open under it
  readonly folderOpen?: boolean;
  readonly sub: ReactNode;
  readonly actions: ReactNode;
  readonly onSelect?: (selected: boolean) => void;
  readonly onTitleClick?: MouseEventHandler<HTMLAnchorElement>;
  readonly onTitleKeyDown?: KeyboardEventHandler<HTMLAnchorElement>;
}

// The words its title shows to the pointer: all that the row may cut or leave out, as the title's hit area covers the
// row and no title of what lies under it shows. A folder's times are its link's, which say nothing of what is in it
const titleWords = (a: Page): string =>
  [
    a.title,
    a.description,
    a.project,
    a.path,
    !a.folder && a.createdAt && t(a.path ? 'ui_registered_at' : 'ui_created_at', fmt(a.createdAt)),
    !a.folder && t('ui_updated_at', fmt(a.updatedAt)),
  ]
    .filter(Boolean)
    .join('\n');

// ReadMark is a row's dot (unread) or ring
export const ReadMark = ({ unread }: { readonly unread: boolean }) =>
  unread ? (
    <span data-testid="dot" className="size-2 flex-none rounded-full bg-new" />
  ) : (
    <span data-testid="ring" className="size-2 flex-none rounded-full border-[1.5px] border-box-line" />
  );

// What the row is: a folder's or a link's mark, or a page's dot (unread) or ring
export function Lead({ page, unread }: { readonly page: Page; readonly unread: boolean }) {
  if (page.folder) return <FolderMark />;
  if (page.path) return <LinkMark />;
  return <ReadMark unread={unread} />;
}

// PageRow is a row of a page, a link or a folder, as the list, a task's page and the search draw it, in the card every
// row of #list is (RowCard)
export function PageRow({
  page: a,
  index,
  picked = false,
  selected = false,
  unread = false,
  archived = false,
  kept = false,
  folderOpen = false,
  sub,
  actions,
  onSelect,
  onTitleClick,
  onTitleKeyDown,
}: PageRowProps) {
  const title = a.title ?? a.slug;
  return (
    <RowCard
      rowKey={`entry:${a.slug}`}
      data={{ 'data-slug': a.slug, 'data-kind': kindOf(a) }}
      index={index}
      picked={picked}
      unread={unread}
      archived={archived}
      gone={Boolean(a.missing)}
      kept={kept}
      select={
        // A row of no selection (a task's page) draws no box
        onSelect ? (
          <label
            data-testid="pick"
            className="group/pick relative z-1 hidden h-[1.4em] cursor-pointer place-items-center [grid-area:lead] in-data-selecting:grid"
          >
            <input
              type="checkbox"
              data-testid="sel"
              checked={selected}
              aria-label={t('ui_select_named', title)}
              onChange={(e) => {
                onSelect(e.currentTarget.checked);
              }}
              className="m-0 size-4 cursor-pointer appearance-none rounded-small border-[1.5px] border-box-line bg-panel transition-colors duration-120 group-hover/pick:border-ink checked:border-transparent checked:bg-ink checked:[background-image:var(--tick)] checked:[background-size:12px] checked:[background-position:center] checked:[background-repeat:no-repeat]"
            />
          </label>
        ) : null
      }
      lead={<Lead page={a} unread={unread} />}
      title={{
        href: a.missing ? undefined : pageHref({ slug: a.slug }),
        words: title,
        tip: titleWords(a),
        expanded: a.folder ? folderOpen : undefined,
        onClick: onTitleClick,
        onKeyDown: onTitleKeyDown,
      }}
      time={a.folder ? '' : a.updatedAt}
      sub={sub}
      actions={actions}
    />
  );
}
