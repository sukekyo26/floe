// The URLs of a page. These alone write them: the frame URL of an entry, or of a folder's page (file), and the page
// alone at its own URL. v is a version of a stored page other than the latest

// The pages' API: GET lists them, and /{slug} and what is under it is one page's
export const PAGES_API = '/api/pages';
export const TASKS_PATH = '/tasks';

export interface PageAt {
  slug: string;
  file?: string;
  v?: number;
  // The page of version v marked against version from, of the pair /diff answered with key
  from?: number;
  key?: string;
}

export const pageHref = (x: PageAt): string =>
  x.file ? `/a/${x.slug}/?file=${encodeURIComponent(x.file)}` : `/a/${x.slug}/${x.v ? `?v=${x.v}` : ''}`;

export const rawHref = (x: PageAt): string =>
  x.file
    ? `/a/${x.slug}/${x.file.split('/').map(encodeURIComponent).join('/')}`
    : `/a/${x.slug}/?raw${x.v ? `&v=${x.v}` : ''}${x.from ? `&from=${x.from}&key=${encodeURIComponent(x.key ?? '')}` : ''}`;

export const diffHref = (slug: string, from?: number, to?: number): string =>
  `/a/${slug}/?diff${from ? `&from=${from}&to=${to ?? ''}` : ''}`;

export const downloadHref = (x: PageAt): string => `${PAGES_API}/${x.slug}/download${x.v ? `?v=${x.v}` : ''}`;

// A task's URL: by the query, as / may be in either part of its name
export const taskHref = (task: { project: string; name: string }): string =>
  `${TASKS_PATH}?${new URLSearchParams({ project: task.project, task: task.name }).toString()}`;
