import type { Query } from '@/stores/list-query';
import type { Page } from '@/types/page';

const PICKS = ['view', 'kind', 'template', 'project', 'tag', 'q'] as const;

// seenOnly tells a change of the list's query that is a page of the view "new" kept as seen, and nothing else: the
// other fields the same, and the seen all the ones before and more. Any other change, the same values written again too
// (the bar's new pages, a search typed again), lets the selection go
export const seenOnly = (was: Query, now: Query): boolean =>
  PICKS.every((key) => was[key] === now[key]) &&
  now.seen.size > was.seen.size &&
  [...was.seen].every((slug) => now.seen.has(slug));

// bulkKind is what removing the selected does: unlink when every one is a link of the list, delete otherwise (a page,
// or one the list no longer has)
export function bulkKind(selected: Iterable<string>, items: readonly Page[]): 'unlink' | 'delete' {
  const paths = new Map(items.map((a) => [a.slug, a.path]));
  return [...selected].every((slug) => paths.get(slug)) ? 'unlink' : 'delete';
}
