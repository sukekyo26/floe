import { t } from '@/lib/t';
import type { TaskOf } from '@/types/task';

// What the task edit shows of a task, as read: its title, its URLs once each, and its categories in their order
export interface EditBase {
  title: string;
  urls: string[];
  categories: string[];
}

export const editBase = (task: Pick<TaskOf, 'title' | 'urls' | 'categories'>): EditBase => ({
  title: task.title ?? '',
  urls: [...new Set(task.urls ?? [])],
  categories: task.categories ?? [],
});

// pagedOf is the task's categories with a page on them, which the store refuses to leave out (ui.html pagedOf)
export const pagedOf = (task: Pick<TaskOf, 'pages' | 'archived'>): string[] => [
  ...new Set([...task.pages, ...task.archived].map((a) => a.task?.category ?? '').filter(Boolean)),
];

const same = (x: readonly string[], y: readonly string[]) => x.join('\n') === y.join('\n');

// mergeLists is the categories or URLs of the modal as the task is read again (now): unchanged, as read; else as the
// person left them (mine), without those another removed and with those another added, after them. One the person
// removed stays out
export function mergeLists(mine: readonly string[], was: readonly string[], now: readonly string[]): string[] {
  if (same(mine, was)) return [...now];
  return [
    ...mine.filter((x) => !was.includes(x) || now.includes(x)),
    ...now.filter((x) => !was.includes(x) && !mine.includes(x)),
  ];
}

// editedURLs is the URLs as saving sends them when their order is not changed: those read, without those taken off,
// and those added after them
export const editedURLs = (read: readonly string[], mine: readonly string[]): string[] => [
  ...read.filter((u) => mine.includes(u)),
  ...mine.filter((u) => !read.includes(u)),
];

// mergeURLs is the URLs of the modal as the task is read again: as read now, with the person's changes, when they did
// not reorder them, so that an order another saved meanwhile stays; else as mergeLists has them
export function mergeURLs(mine: readonly string[], was: readonly string[], now: readonly string[]): string[] {
  if (!same(mine, editedURLs(was, mine))) return mergeLists(mine, was, now);
  const added = mine.filter((u) => !was.includes(u));
  return [...now.filter((u) => !was.includes(u) || mine.includes(u)), ...added.filter((u) => !now.includes(u))];
}

// The modal's fields as they are now
export interface EditNow {
  title: string;
  urls: string[];
  categories: string[];
}

// editChange is what saving sends of what was changed since base: the title, the URLs added, taken off and, when they
// are reordered, their order, and the categories in their order; null where nothing was changed. Only what the person
// changed, so that a URL another added or removed meanwhile is left as it is (ui.md: tasks' URLs)
export function editChange(base: EditBase, now: EditNow): Record<string, unknown> | null {
  const title = now.title.trim();
  const added = now.urls.filter((u) => !base.urls.includes(u));
  const removed = base.urls.filter((u) => !now.urls.includes(u));
  const ordered = !same(now.urls, editedURLs(base.urls, now.urls));
  const urls = {
    ...(added.length > 0 ? { add: added } : {}),
    ...(removed.length > 0 ? { remove: removed } : {}),
    ...(ordered ? { order: now.urls } : {}),
  };
  const changed = {
    ...(title === base.title ? {} : { title }),
    ...(Object.keys(urls).length > 0 ? { urls } : {}),
    ...(same(now.categories, base.categories) ? {} : { categories: now.categories }),
  };
  return Object.keys(changed).length > 0 ? changed : null;
}

// refillTitle is the title field once the task is read again: what was typed stays, or, left as it opened, the title
// read now
export const refillTitle = (typed: string, was: string, now: string): string => (typed.trim() === was ? now : typed);

// urlRefusal is why a URL typed is not added: none, one there already, or one past the most a task holds; '' to add
// it. Whether it is a URL at all the store says, as it saves
export function urlRefusal(url: string, urls: readonly string[], max: number): string {
  if (!url) return t('ui_url_blank');
  if (urls.includes(url)) return t('ui_url_there', url);
  if (urls.length >= max) return t('ui_url_max', max);
  return '';
}

// categoryRefusal is why a category's name typed is not added: none, or one there already; '' to add it
// LIMIT: trim and Go's TrimSpace drop a few spaces apart (U+0085, U+FEFF): two names this takes to differ by them,
// categoriesOf makes one
export function categoryRefusal(name: string, names: readonly string[]): string {
  if (!name) return t('ui_category_blank');
  if (names.includes(name)) return t('ui_category_there', name);
  return '';
}

// leftOver is what stops saving of words typed in a field to add and not added: there already, or not added yet; ''
// for none
export function leftOver(typed: string, names: readonly string[], url: boolean): string {
  if (!typed) return '';
  if (names.includes(typed)) return t(url ? 'ui_url_there' : 'ui_category_there', typed);
  return t(url ? 'ui_url_unadded' : 'ui_category_unadded', typed);
}
