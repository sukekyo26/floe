import { getBoot } from '@/config/boot';
import { projectKept as kept0, type ProjectKept } from '@/features/tasks/stores/project-kept';
import { listState, openProject, pinnedOf, shownTasks, type ListState } from '@/features/tasks/utils/task-list';
import { useStore } from '@/hooks/use-store';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';
import { knownTasks } from '@/utils/tasks-of';

export interface TaskListView {
  read: boolean;
  // failed tells the tasks unreadable, why the error
  failed: boolean;
  error: string;
  tasks: TaskOf[];
  entries: Page[];
  // project and view are what the list shows: the project asked for, or the one it opens with (openProject)
  project: string;
  view: string;
  shown: TaskOf[];
  pinned: Page[];
  state: ListState;
}

// Before the first read nothing is said: no words of no task, no way to add one
const UNREAD: ListState = { nothing: false, none: false, gone: false, addHidden: true };

export interface TaskListDeps {
  query?: ListQuery;
  pages?: Pages;
  kept?: ProjectKept;
}

// useTaskList is what the task list shows, as the list's query and the list read are now, taken once for the screen's
// shelf and rows alike
export function useTaskList({ query = listQuery, pages = pagePages, kept = kept0 }: TaskListDeps = {}): TaskListView {
  const { project: asked, view } = useStore(query.query);
  const { read, entries, tasks: taken } = useStore(pages.state);
  const { lang, states } = getBoot();
  const failed = Boolean(taken.error);
  // Unreadable, none is drawn, and only what was kept can be said of the project
  const tasks = knownTasks(entries, taken);
  const first = states.find((s) => s.first)?.name ?? '';
  const project = openProject(asked, { tasks, entries, kept: kept.kept(), failed, first, lang });
  const shown = shownTasks(tasks, project, view, lang);
  return {
    read,
    failed,
    error: taken.error,
    tasks,
    entries,
    project,
    view,
    shown,
    pinned: pinnedOf(entries, project),
    state: read ? listState(tasks, entries, project, view, failed, shown.length) : UNREAD,
  };
}
