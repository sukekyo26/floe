import { unseenSlugs } from '@/features/page-list/utils/list-shelf';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';

// keepNewSeen keeps every unread page as seen while the view "new" shows, so that a page read stays on it: at once, at
// each read of the list, and at each pick, which lets the seen go even when the view stays "new" (the bar's new pages).
// It returns what stops it
export function keepNewSeen(query: ListQuery = listQuery, pages: Pages = pagePages): () => void {
  const keep = () => {
    if (query.query.get().view === 'new') query.see(unseenSlugs(pages.state.get().entries));
  };
  keep();
  const stops = [query.query.subscribe(keep), pages.state.subscribe(keep)];
  return () => {
    for (const stop of stops) stop();
  };
}
