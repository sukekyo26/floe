import { leftPlace } from '@/features/tasks/utils/task-set';
import { t } from '@/lib/t';
import type { PageWritten } from '@/lib/tasks';
import type { Page } from '@/types/page';
import { searched } from '@/utils/search';

// The task and category Add a page puts a page on, and the rev it was opened at, or read again at
export interface AddTo {
  project: string;
  name: string;
  category: string;
  rev: number;
}

// addHits are the pages Add a page offers: of the task's project, not archived, not on that category of the task
// already, those the words match
// LIMIT: for no category, a page of the task's on a category it has not (a hand edit) is offered too: put, it is drawn
// where it was, in no category's group, said to be moved. The store keeps a page's category among its task's, so only a
// hand edit leaves one
export function addHits(entries: readonly Page[], to: AddTo, words: string): { all: Page[]; hits: Page[] } {
  const all = entries.filter(
    (a) =>
      a.taskProject === to.project &&
      !a.archived &&
      !(a.task?.name === to.name && (a.task.category ?? '') === to.category),
  );
  return { all, hits: searched(words, all) };
}

// groupName is a group's heading on a task's page: its category, or the words for none
// LIMIT: a category named as those words is told from no category's group only by its key and its add button's name
export const groupName = (category: string): string => category || t('ui_category_none');

// hereBadge is what a row says of the task a page is on: this task's category, or another task's and category; ''
// for none
export function hereBadge(a: Page, to: Pick<AddTo, 'name'>): string {
  const name = a.task?.name;
  if (!name) return '';
  if (name === to.name) {
    // An empty category is none, as a missing one is
    const category = a.task?.category ?? '';
    return t('ui_page_here_category', category === '' ? t('ui_page_from_no_category') : category);
  }
  return [name, a.task?.category].filter(Boolean).join(' · ');
}

// pageFrom is where a page comes from, as said of it: this task's other category, another task's, or none
export function pageFrom(a: Page, to: Pick<AddTo, 'name'>): string {
  const { name, category } = a.task ?? {};
  if (!name) return '';
  if (name === to.name) return category ? t('ui_page_from_category', category) : t('ui_page_from_no_category');
  return leftPlace(name, category);
}

// addSaid is what is said once a is put: moved from another task, as the write found it, or from this task's other
// category, as the list had it (from, pageFrom before the write); else added. The list's other task is not said: the
// write found the page off it
export function addSaid(
  a: Page,
  to: Pick<AddTo, 'name' | 'category'>,
  from: string,
  off: PageWritten | undefined,
): string {
  const said = off?.offTask ? leftPlace(off.offTask, off.offCategory) : a.task?.name === to.name ? from : '';
  const title = a.title ?? a.slug;
  if (!to.category) return said ? t('ui_page_moved_no_category', title, said) : t('ui_page_added_no_category', title);
  return said ? t('ui_page_moved', title, said, to.category) : t('ui_page_added', title, to.category);
}
