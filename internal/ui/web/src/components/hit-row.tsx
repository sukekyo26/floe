import type { MouseEvent, ReactNode } from 'react';

import { Lead } from '@/components/page-row';
import { ago } from '@/lib/time';
import type { Page } from '@/types/page';
import { unseen } from '@/utils/list-view';
import { pageHref } from '@/utils/page-urls';

interface HitRowProps {
  readonly id: string;
  readonly page: Page;
  readonly reached: boolean;
  // sub is the line under the title (RowSub)
  readonly sub: ReactNode;
  // onOpen is a plain press of the title's link, which goes on to the page
  readonly onOpen?: () => void;
  // link false: the title is words, not a link, where choosing the row does what it does (Add a page of a task)
  readonly link?: boolean;
}

const TITLE =
  "min-w-0 truncate font-medium text-fg no-underline [grid-area:title] group-data-unread/hit:font-bold group-data-archived/hit:text-muted after:absolute after:inset-0 after:content-['']";

// HitRow is a row of a search modal's list (the search, and Add a page of a task): an option of a listbox, the whole
// row its title's link, out of the Tab order as the field moves over the rows
export function HitRow({ id, page: a, reached, sub, onOpen, link = true }: HitRowProps) {
  const time = a.folder ? '' : a.updatedAt;
  const fresh = unseen(a);
  return (
    <li
      id={id}
      // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role, jsx-a11y/no-noninteractive-element-to-interactive-role -- an option of the listbox holding a link, which <option> cannot hold
      role="option"
      aria-selected={reached}
      data-unread={fresh || undefined}
      data-archived={a.archived ? true : undefined}
      className="group/hit relative grid grid-cols-[1.1rem_minmax(0,1fr)_auto] items-center gap-x-2.25 gap-y-0.5 rounded-small px-3 py-1.75 [grid-template-areas:'lead_title_time'_'._sub_sub'] aria-selected:bg-sel"
    >
      <span data-testid="lead" className="grid place-items-center text-muted [grid-area:lead] [&>svg]:size-3.5">
        <Lead page={a} unread={fresh} />
      </span>
      {link ? (
        <a
          data-testid="title"
          tabIndex={-1}
          href={pageHref(a)}
          title={a.title ?? a.slug}
          onClick={(e: MouseEvent) => {
            if (!e.ctrlKey && !e.metaKey && !e.shiftKey) onOpen?.();
          }}
          className={TITLE}
        >
          {a.title ?? a.slug}
        </a>
      ) : (
        <span data-testid="title" title={a.title ?? a.slug} className={TITLE}>
          {a.title ?? a.slug}
        </span>
      )}
      <span className="flex items-center gap-1.5 font-mono text-label leading-none whitespace-nowrap text-muted [grid-area:time] [&>svg]:size-3 [&>svg]:text-star">
        {a.favorite ? (
          <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z" />
          </svg>
        ) : null}
        <time dateTime={time}>{ago(time)}</time>
      </span>
      <span
        data-testid="sub"
        className="flex min-w-0 flex-nowrap items-center gap-x-1.5 text-label text-muted [grid-area:sub] [&>:not([data-testid=path])]:flex-none"
      >
        {sub}
      </span>
    </li>
  );
}
