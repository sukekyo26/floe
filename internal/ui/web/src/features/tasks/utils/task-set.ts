import { stateWord } from '@/features/tasks/utils/kin-forms';
import { t } from '@/lib/t';
import type { TasksAnswer, TasksChange } from '@/lib/tasks';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';
import { taskHref } from '@/utils/page-urls';

// leftPlace is where a page was, as said of it: its task, and the category on it when it had one
export const leftPlace = (task: string, category: string | undefined): string =>
  category ? t('ui_page_from_task_category', task, category) : task;

// moveLead is what the move's question says of the page as the list has it now (listed), put on the task to
export function moveLead(listed: Pick<Page, 'slug' | 'title' | 'task'>, to: string): string {
  const from = listed.task?.name?.trim() ?? '';
  return t('ui_task_move_lead', listed.title ?? listed.slug, leftPlace(from, listed.task?.category), to.trim(), from);
}

// movesFrom is the task a page is on, as the list has it, that putting it on to would take it off: '' when none, as on
// none or on that one
export function movesFrom(listed: Pick<Page, 'task'>, to: string): string {
  const from = listed.task?.name?.trim() ?? '';
  return from === to.trim() ? '' : from;
}

// setBody is what Set the task says of the task named, as the modal shows it (shown): one there by its rev, a new one
// with its title
export const setBody = (shown: TaskOf | undefined, title: string): TasksChange =>
  shown ? { new: false, rev: shown.rev } : { new: true, title: title.trim() };

// What Set the task says once the page is put: saved, where it stays on the task it opened on, else a link to the task
export type SetSaid = { text: string } | { link: string; href: string };

// setSaid is what is said once the page is put on name: the task it left as the write found it, the state it comes to
// be under (the task's as shown, or a new task's), and the name it was opened with (opened)
export function setSaid(
  page: Pick<Page, 'taskProject'>,
  name: string,
  opened: string,
  answer: Pick<TasksAnswer, 'pages'>,
  state: string,
): SetSaid {
  const to = name.trim();
  const off = answer.pages[0];
  if (!off?.offTask && to === opened) return { text: t('ui_saved') };
  const link = off?.offTask
    ? t('ui_task_put_moved', leftPlace(off.offTask, off.offCategory), to, stateWord(state))
    : t('ui_task_put', to, stateWord(state));
  return { link, href: taskHref({ project: page.taskProject, name: to }) };
}
