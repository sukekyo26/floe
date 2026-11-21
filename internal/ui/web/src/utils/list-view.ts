import type { Page } from '@/types/page';

// NONE is the project of pages from before the project was recorded: a project is a directory name without /
export const NONE = '/';

// The list's state, as the URL keeps it
export interface ListState {
  view: string;
  kind: string;
  template: string;
  project: string;
  tag: string;
  q: string;
}

// kindOf is what an entry is: a folder's link, any other link (by absolute path), or a stored page
export const kindOf = (a: Page): 'folder' | 'link' | 'page' => (a.folder ? 'folder' : a.path ? 'link' : 'page');

// inView tells whether a is of the view of state: the archived on the archive's alone. seen are the pages of the view
// "new" seen since it was picked, which stay on it. Every count and list goes through this
export const inView = (a: Page, state: Pick<ListState, 'view'>, seen: ReadonlySet<string>): boolean =>
  Boolean(a.archived) === (state.view === 'archived') &&
  (state.view !== 'fav' || Boolean(a.favorite)) &&
  (state.view !== 'new' || Boolean(a.news) || seen.has(a.slug));

// matches tells whether an entry has the words searched (lowercased) in its title, description, slug, project, path,
// template or tags; any, for none
export const matches = (a: Page, q: string): boolean =>
  !q ||
  [a.title, a.description, a.slug, a.project, a.path, a.template, ...(a.tags ?? [])]
    .join(' ')
    .toLowerCase()
    .includes(q);

// arrange picks the entries by the shelf and the search, in the server's order: the newest first. Shared by the list
// and by the pages j and k go through in the frame
export function arrange(items: readonly Page[], state: ListState, seen: ReadonlySet<string>): Page[] {
  const q = state.q.trim().toLowerCase();
  return items.filter(
    (a) =>
      inView(a, state, seen) &&
      (!state.kind || kindOf(a) === state.kind) &&
      (!state.template || a.template === state.template) &&
      (!state.project || (a.project ?? NONE) === state.project) &&
      (!state.tag || (a.tags ?? []).includes(state.tag)) &&
      matches(a, q),
  );
}

// unseen is what the count and the marks of a row have as unseen: an archived page is not, as nothing counts it
export const unseen = (a: Page): boolean => Boolean(a.news) && !a.archived;

// hasPage tells whether a has a page of its own to open alone or download: a folder has none, nor a link whose file
// is gone
export const hasPage = (a: Page): boolean => !a.folder && !a.missing;

export const canDiff = (a: Page): boolean => !a.path && (a.versionCount ?? 0) >= 2;

// removal is what removing a says: a link's file stays, so it is unlinked rather than deleted
export const removal = (a: Page): 'unlink' | 'delete' => (a.path ? 'unlink' : 'delete');
