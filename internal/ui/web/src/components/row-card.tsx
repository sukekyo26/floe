import type { CSSProperties, KeyboardEventHandler, MouseEventHandler, ReactNode } from 'react';

import { Ago } from '@/components/ago';
import type { CssVars } from '@/utils/css-vars';

export interface RowTitle {
  // href is left out where the row has nothing to open (a link whose file is gone): it then takes no focus by Tab
  readonly href: string | undefined;
  readonly words: ReactNode;
  // tip is all the row may cut or leave out, shown to the pointer over the whole row
  readonly tip: string;
  readonly expanded?: boolean | undefined;
  readonly onClick?: MouseEventHandler<HTMLAnchorElement> | undefined;
  readonly onKeyDown?: KeyboardEventHandler<HTMLAnchorElement> | undefined;
}

export interface RowCardProps {
  readonly rowKey: string;
  // data are the row's own attributes (data-slug, data-kind, …), drawn after data-key
  readonly data?: Readonly<Record<string, string | undefined>>;
  // index is its place in the list, which staggers its rise
  readonly index: number;
  // picked: open beside the list; unread: bold; archived: dimmed; gone: struck through; kept: drawn before, so not
  // risen in again; fresh: just made (a task's)
  readonly picked?: boolean;
  readonly unread?: boolean;
  readonly archived?: boolean;
  readonly gone?: boolean;
  readonly kept?: boolean;
  readonly fresh?: boolean;
  // select is the row's box, left out where nothing is selected (a task's page, the task list)
  readonly select?: ReactNode;
  readonly lead: ReactNode;
  readonly title: RowTitle;
  // time is when it changed, '' for none (a folder)
  readonly time: string;
  readonly sub: ReactNode;
  // actions are the row's buttons, left out where it has none (the task list)
  readonly actions?: ReactNode;
}

// RowCard is a row of #list, of whatever it lists: a card whose title's hit area covers it, the controls over it. Its
// state is in its attributes
export function RowCard({
  rowKey,
  data = {},
  index,
  picked = false,
  unread = false,
  archived = false,
  gone = false,
  kept = false,
  fresh = false,
  select,
  lead,
  title,
  time,
  sub,
  actions,
}: RowCardProps) {
  const style: CssVars = { '--i': Math.min(index, 30) };
  return (
    <li
      data-key={rowKey}
      {...data}
      aria-current={picked || undefined}
      data-unread={unread || undefined}
      data-archived={archived || undefined}
      data-gone={gone || undefined}
      data-kept={kept || undefined}
      data-fresh={fresh || undefined}
      style={style as CSSProperties}
      className="group/row relative grid animate-rise grid-cols-[12px_minmax(0,1fr)_auto_auto] gap-x-[0.65rem] gap-y-[0.2rem] rounded-menu border border-line bg-panel px-4 py-[0.45rem] shadow-sm transition-[background-color,box-shadow] duration-150 [animation-delay:calc(var(--i,0)*16ms)] [grid-template-areas:'lead_title_title_time'_'._sub_actions_actions'] not-first:mt-1.5 hover:bg-hover has-checked:bg-sel has-checked:hover:bg-sel has-[[data-testid=title]:focus-visible]:outline-2 has-[[data-testid=title]:focus-visible]:-outline-offset-2 has-[[data-testid=title]:focus-visible]:outline-faint has-[details[open]]:z-3 has-[+li[data-testid=tree]]:animate-none has-[+li[data-testid=tree]]:rounded-b-none has-[+li[data-testid=tree]]:border-b-0 aria-current:border-new aria-current:not-has-checked:bg-new-soft aria-current:not-has-checked:hover:bg-new-soft data-fresh:bg-sel data-fresh:hover:bg-sel data-kept:animate-none can-hover:not-has-[+li[data-testid=tree]]:hover:shadow-md [[data-testid=group]+&]:mt-0"
    >
      {select}
      <span
        data-testid="lead"
        className="grid h-[1.4em] place-items-center text-muted [grid-area:lead] in-data-selecting:hidden [&>svg]:size-3.5"
      >
        {lead}
      </span>
      <a
        data-testid="title"
        href={title.href}
        // Without href it cannot take focus and ↑↓ would stop before this row, so scripts can still move to it
        tabIndex={title.href === undefined ? -1 : undefined}
        title={title.tip}
        aria-expanded={title.expanded}
        onClick={title.onClick}
        onKeyDown={title.onKeyDown}
        className="line-clamp-2 min-w-0 scroll-mt-[calc(var(--spacing-lhead)+0.5rem)] scroll-mb-12 font-medium text-fg no-underline [grid-area:title] group-data-archived/row:text-muted group-data-gone/row:text-muted group-data-gone/row:line-through group-data-unread/row:font-bold after:absolute after:inset-0 focus-visible:outline-none"
      >
        {title.words}
      </a>
      <time
        dateTime={time}
        className="text-right font-mono text-label leading-[1.9] whitespace-nowrap text-muted tabular-nums [grid-area:time]"
      >
        <Ago iso={time} />
      </time>
      <span
        data-testid="sub"
        className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-label text-muted [grid-area:sub]"
      >
        {sub}
      </span>
      {actions === undefined ? null : (
        <div
          data-testid="actions"
          className="relative z-1 flex items-center gap-0.5 [grid-area:actions] can-hover:[&>*]:opacity-0 can-hover:[&>*]:transition-opacity can-hover:[&>*]:duration-120 can-hover:group-focus-within/row:[&>*]:opacity-100 can-hover:group-hover/row:[&>*]:opacity-100 can-hover:group-has-checked/row:[&>*]:opacity-100 can-hover:group-has-[details[open]]/row:[&>*]:opacity-100 can-hover:group-aria-current/row:[&>*]:opacity-100 can-hover:[&>[aria-pressed=true]]:opacity-100"
        >
          {actions}
        </div>
      )}
    </li>
  );
}
