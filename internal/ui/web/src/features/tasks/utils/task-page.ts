import { taskOrder, unreadOf } from '@/features/tasks/utils/task-list';
import type { Page } from '@/types/page';
import type { TaskOf, TaskRow } from '@/types/task';
import { TASKS_PATH } from '@/utils/page-urls';

export interface PageTaskOf {
  // found is the task as read; rows, the project's rows as the store has them (a write's answer goes in while tasks.json
  // is unreadable)
  found: TaskOf | undefined;
  rows: Readonly<Record<string, TaskRow>> | undefined;
  failed: boolean;
  // last is the task as last read, null before any
  last: TaskOf | null;
}

export interface PageTask {
  // task is what the page draws, null for none: missing, or unreadable before it was ever read
  task: TaskOf | null;
  // none: no page on it, archived or not; missing: read, and not there
  none: boolean;
  missing: boolean;
}

// asWritten is the task last read as the screen wrote it since, while tasks.json is unreadable: its own row as answered
// (a parent taken off is left out of the answer), its parent's row kept while the same, its children those whose rows
// as answered still name it. With its pages as they are now
// LIMIT: the rows answered go as the list is read anew with other words (pages.ts takes the unreadable tasks then), and
// it is drawn as last read again, its next write refused as stale. Keep the rows answered over such a read if that
// comes to trouble someone
function asWritten(last: TaskOf, found: TaskOf | undefined, rows: PageTaskOf['rows']): TaskOf {
  const own = rows?.[last.name];
  const { title: _title, urls: _urls, categories: _categories, parent: _parent, ...kept } = last;
  const { parentTask, ...task } = own ? { ...kept, ...own, rev: own.rev ?? last.rev } : last;
  return {
    ...task,
    ...(parentTask && (!own || own.parent === last.parent) ? { parentTask } : {}),
    children: last.children.filter((x) => rows?.[x.name] === undefined || rows[x.name]?.parent === last.name),
    pages: found?.pages ?? [],
    archived: found?.archived ?? [],
  };
}

// pageTask is the task of a task's page. Unreadable, it is as last read, with what the screen wrote since and its pages
// as they are now; what is not known is not said to be missing
export function pageTask({ found, rows, failed, last }: PageTaskOf): PageTask {
  const task = failed ? (last ? asWritten(last, found, rows) : null) : (found ?? null);
  return {
    task,
    none: task?.pages.length === 0 && task.archived.length === 0,
    missing: !failed && !found,
  };
}

// A group of a task's page's rows: its category, '' for no category, and its pages
export interface Group {
  category: string;
  entries: Page[];
}

const madeAt = (a: Page): number => Date.parse(a.createdAt || a.updatedAt);

// taskGroups is the rows of a task's page: first no category, with what is on no category of the task's, as a hand
// edit leaves, where it has pages; then each category in its order, with no page too. Within each, what is archived
// after the rest, each as first published
export function taskGroups(task: TaskOf): Group[] {
  const order = task.categories ?? [];
  const none: Page[] = [];
  const by = new Map<string, Page[]>(order.map((heading) => [heading, []]));
  const pages = [...task.pages, ...task.archived].sort(
    (x, y) => Number(Boolean(x.archived)) - Number(Boolean(y.archived)) || madeAt(x) - madeAt(y),
  );
  for (const a of pages) (by.get(a.task?.category ?? '') ?? none).push(a);
  const groups = order.map((category) => ({ category, entries: by.get(category) ?? [] }));
  return none.length > 0 ? [{ category: '', entries: none }, ...groups] : groups;
}

export interface Family {
  // ups: the parent's part shows (no children); adds: the way to add a child shows (no parent)
  ups: boolean;
  adds: boolean;
  // up is the parent's row, kids the children's, by state then the latest first
  up: TaskOf | undefined;
  kids: TaskOf[];
  // kidsShown and shown: the children's part, and the card
  kidsShown: boolean;
  shown: boolean;
}

// familyOf is a task's family card: its parent, offered while it has no children, and its children, offered while it
// has no parent. What the store's checkParent would refuse is not offered
export function familyOf(task: TaskOf, states: readonly string[], lang: string): Family {
  const at = (x: TaskOf) => states.indexOf(x.state);
  const kids = [...task.children].sort((x, y) => at(x) - at(y) || taskOrder(lang)(x, y));
  const ups = task.children.length === 0;
  const adds = !task.parent;
  return {
    ups,
    adds,
    up: task.parentTask,
    kids,
    kidsShown: kids.length > 0 || adds,
    shown: ups || kids.length > 0 || adds,
  };
}

// familySaid is all the card says, revs too: its rows are made anew as it changes, and kept as it does not (the key of
// each)
export const familySaid = (task: TaskOf, family: Family): string =>
  JSON.stringify([
    task.name,
    task.parent,
    task.rev,
    [family.up, ...family.kids].map((x) => x && [x.name, x.title, x.parent, x.state, x.rev, unreadOf(x) > 0]),
  ]);

// taskListHref is the task list of a task's page: the project's, at the view the task is under now, where it is to be
// found whatever way it was come to; with no task read, the project's first view
export function taskListHref(project: string, state: string | undefined, first: string): string {
  const to = new URLSearchParams({ project });
  if (state !== undefined && state !== first) to.set('view', state);
  return `${TASKS_PATH}?${to.toString()}`;
}
