import type { CSSProperties, MouseEventHandler, ReactNode } from 'react';

import { FolderMark, LinkMark } from '@/components/icons';
import { t } from '@/lib/t';
import { ago, fmt } from '@/lib/time';
import type { Page } from '@/types/page';
import type { CssVars } from '@/utils/css-vars';
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

// What the row is: a folder's or a link's mark, or a page's dot (unread) or ring
export function Lead({ page, unread }: { readonly page: Page; readonly unread: boolean }) {
  if (page.folder) return <FolderMark />;
  if (page.path) return <LinkMark />;
  return unread ? (
    <span data-testid="dot" className="size-2 flex-none rounded-full bg-new" />
  ) : (
    <span data-testid="ring" className="size-2 flex-none rounded-full border-[1.5px] border-box-line" />
  );
}

// PageRow is a row of a page, a link or a folder, as the list, a task's page and the search draw it (ui.html template
// #row): a card whose title's hit area covers it, the controls over it. Its state is in its attributes
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
}: PageRowProps) {
  const title = a.title ?? a.slug;
  const time = a.folder ? '' : a.updatedAt;
  const style: CssVars = { '--i': Math.min(index, 30) };
  return (
    <li
      data-key={`entry:${a.slug}`}
      data-slug={a.slug}
      aria-current={picked || undefined}
      data-unread={unread || undefined}
      data-archived={archived || undefined}
      data-gone={a.missing ? true : undefined}
      data-kept={kept || undefined}
      style={style as CSSProperties}
      className="group/row relative grid animate-rise grid-cols-[12px_minmax(0,1fr)_auto_auto] gap-x-2.5 gap-y-0.75 rounded-menu border border-line bg-panel px-4 py-1.75 shadow-sm transition-[background-color,box-shadow] duration-150 [animation-delay:calc(var(--i,0)*16ms)] [grid-template-areas:'lead_title_title_time'_'._sub_actions_actions'] not-first:mt-1.5 hover:bg-hover has-checked:bg-sel has-checked:hover:bg-sel has-[a:focus-visible]:outline-2 has-[a:focus-visible]:-outline-offset-2 has-[a:focus-visible]:outline-faint has-[details[open]]:z-3 aria-current:border-new aria-current:not-has-checked:bg-new-soft aria-current:not-has-checked:hover:bg-new-soft data-kept:animate-none can-hover:hover:shadow-md [[data-testid=group]+&]:mt-0"
    >
      <label
        data-testid="pick"
        className="group/pick relative z-1 hidden h-[1.4em] cursor-pointer place-items-center [grid-area:lead] in-data-selecting:grid"
      >
        <input
          type="checkbox"
          data-testid="sel"
          checked={selected}
          aria-label={t('ui_select_named', title)}
          onChange={(e) => onSelect?.(e.currentTarget.checked)}
          className="m-0 size-4 cursor-pointer appearance-none rounded-small border-[1.5px] border-box-line bg-panel transition-colors duration-120 group-hover/pick:border-ink checked:border-transparent checked:bg-ink checked:[background-image:var(--tick)] checked:[background-size:12px] checked:[background-position:center] checked:[background-repeat:no-repeat]"
        />
      </label>
      <span
        data-testid="lead"
        className="grid h-[1.4em] place-items-center text-muted [grid-area:lead] in-data-selecting:hidden [&>svg]:size-3.5"
      >
        <Lead page={a} unread={unread} />
      </span>
      <a
        data-testid="title"
        href={a.missing ? undefined : pageHref({ slug: a.slug })}
        // Without href it cannot take focus and ↑↓ would stop before this row, so scripts can still move to it
        tabIndex={a.missing ? -1 : undefined}
        title={titleWords(a)}
        aria-expanded={a.folder ? folderOpen : undefined}
        onClick={onTitleClick}
        className="line-clamp-2 min-w-0 scroll-mt-[calc(var(--spacing-lhead)+0.5rem)] scroll-mb-12 font-medium text-fg no-underline [grid-area:title] group-data-archived/row:text-muted group-data-gone/row:text-muted group-data-gone/row:line-through group-data-unread/row:font-bold after:absolute after:inset-0 focus-visible:outline-none"
      >
        {title}
      </a>
      <time
        dateTime={time}
        className="text-right font-mono text-label leading-[1.9] whitespace-nowrap text-muted tabular-nums [grid-area:time]"
      >
        {ago(time)}
      </time>
      <span
        data-testid="sub"
        className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-label text-muted [grid-area:sub]"
      >
        {sub}
      </span>
      <div
        data-testid="actions"
        className="relative z-1 flex items-center gap-0.5 [grid-area:actions] can-hover:[&>*]:opacity-0 can-hover:[&>*]:transition-opacity can-hover:[&>*]:duration-120 can-hover:group-focus-within/row:[&>*]:opacity-100 can-hover:group-hover/row:[&>*]:opacity-100 can-hover:group-has-checked/row:[&>*]:opacity-100 can-hover:group-has-[details[open]]/row:[&>*]:opacity-100 can-hover:group-aria-current/row:[&>*]:opacity-100 can-hover:[&>[aria-pressed=true]]:opacity-100"
      >
        {actions}
      </div>
    </li>
  );
}
