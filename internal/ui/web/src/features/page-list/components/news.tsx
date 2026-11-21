import type { MouseEventHandler } from 'react';

import { Button } from '@/components/button';
import { getBoot } from '@/config/boot';
import { showResults } from '@/features/page-list/stores/show-results';
import { newCount, newsClick, newsPress, newsTaken } from '@/features/page-list/utils/list-shelf';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { columns as pageColumns, type ColumnsStore } from '@/stores/columns';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';

export interface NewsProps {
  // hidden is where it stands: a page found whole has none (ui.html 5215)
  readonly hidden?: boolean;
  readonly query?: ListQuery;
  readonly pages?: Pages;
  readonly columns?: ColumnsStore;
}

// News is the bar's count of new pages and the way to them (ui.html #news, renderNews): every unread page not archived,
// gone at none. On the page list a plain press narrows the list to them in place; on the task screens, and pressed with
// a key held or another button, it is a link to that list. Up to 70rem the shelf's views say it
export function News({ hidden = false, query = listQuery, pages = pagePages, columns = pageColumns }: NewsProps) {
  const n = newCount(useStore(pages.state).entries);
  const onList = newsTaken(getBoot().screen);
  const take: MouseEventHandler<HTMLAnchorElement> = (e) => {
    if (!newsClick(e)) return;
    e.preventDefault();
    const { picks, q } = newsPress();
    query.setWords(q);
    query.filter(picks);
    showResults(columns);
  };
  return (
    <Button
      id="news"
      href="/?view=new"
      hidden={hidden || n === 0}
      hideFrom="wide"
      {...(onList ? { onClick: take } : {})}
    >
      <span data-testid="dot" className="size-2 flex-none rounded-full bg-new" />
      <span id="news-n">{t('ui_news', n)}</span>
    </Button>
  );
}
