import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';

// syncList writes the page list's state into the URL and keeps it for the tab, as it starts, as the list is narrowed
// (before the first read and after a read that failed too, so that a reload keeps it) and as the list is read. It
// returns what stops it
export function syncList(query: ListQuery = listQuery, pages: Pages = pagePages): () => void {
  query.sync();
  const stops = [
    query.query.subscribe(() => {
      query.sync();
    }),
    pages.state.subscribe(() => {
      if (pages.state.get().read) query.sync();
    }),
  ];
  return () => {
    for (const stop of stops) stop();
  };
}
