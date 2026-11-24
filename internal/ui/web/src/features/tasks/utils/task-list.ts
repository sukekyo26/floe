import type { Page } from '@/types/page';
import type { State, TaskOf } from '@/types/task';
import { NONE } from '@/utils/list-view';
import { projectOrder } from '@/utils/project-order';

// taskOrder places tasks by when they were last written (a page come to one too), the latest first, then by name
export const taskOrder =
  (lang: string) =>
  (x: TaskOf, y: TaskOf): number =>
    Date.parse(y.at) - Date.parse(x.at) || x.name.localeCompare(y.name, lang);

// pinnedOf is the pages pinned of a project, which leave as they are archived ('' for every project)
export const pinnedOf = (entries: readonly Page[], project = ''): Page[] =>
  entries.filter((a) => a.task?.pin && !a.archived && (!project || a.taskProject === project));

// taskProjects is the projects there are for the task list: of its tasks, and of the pages pinned
export const taskProjects = (tasks: readonly TaskOf[], entries: readonly Page[]): Set<string> =>
  new Set([...tasks.map((task) => task.project), ...pinnedOf(entries).map((a) => a.taskProject)]);

// taskRanks is the order of the task list's projects and their colours
export const taskRanks = (entries: readonly Page[], there: ReadonlySet<string>): Map<string, number> =>
  projectOrder(entries, there);

export interface Opening {
  tasks: readonly TaskOf[];
  entries: readonly Page[];
  // kept is the viewer's last project (pages.taskProject), null for none
  kept: string | null;
  failed: boolean;
  // first is the state of tasks in progress, the list's first view
  first: string;
  lang: string;
}

// openingProject is the project the task list opens with no project named: the one kept while it is there, or where a
// task in progress was written last, or the shelf's first, or no project. Unreadable, tasks are not known, so that only
// what was kept can be said
export function openingProject({ tasks, entries, kept, failed, first, lang }: Opening): string {
  if (failed) return kept ?? '';
  const there = taskProjects(tasks, entries);
  if (kept !== null && there.has(kept)) return kept;
  const going = tasks.filter((task) => task.state === first).sort(taskOrder(lang));
  return going[0]?.project ?? [...taskRanks(entries, there).keys(), NONE].find((name) => there.has(name)) ?? '';
}

// openProject is the project the task list shows: the one asked for (the URL's, kept by the list's query), else the
// one it opens with
export const openProject = (asked: string, opening: Opening): string => asked || openingProject(opening);

// shownTasks is the task list's rows: the project's tasks of the view, newest first
export const shownTasks = (tasks: readonly TaskOf[], project: string, view: string, lang: string): TaskOf[] =>
  tasks.filter((task) => task.state === view && task.project === project).sort(taskOrder(lang));

// stateCounts is each state's count of the project's tasks, for the shelf's views
export const stateCounts = (
  tasks: readonly TaskOf[],
  project: string,
  states: readonly State[],
): Record<string, number> =>
  Object.fromEntries(
    states.map((s) => [s.name, tasks.filter((task) => task.state === s.name && task.project === project).length]),
  );

export interface ProjectChip {
  name: string;
  // n is how many tasks of the view it has
  n: number;
}

// projectChips is the shelf's projects of the task list: every project there, in the order of their colours; the one
// shown though gone (a name in the URL) after them, and no project last. Each with the tasks it has of the view
export function projectChips(
  tasks: readonly TaskOf[],
  entries: readonly Page[],
  project: string,
  view: string,
): ProjectChip[] {
  const there = taskProjects(tasks, entries);
  const n = (name: string) => tasks.filter((task) => task.state === view && task.project === name).length;
  const names = [...taskRanks(entries, there).keys()].filter((name) => there.has(name));
  if (project && project !== NONE && !names.includes(project)) names.push(project);
  if (there.has(NONE) || project === NONE) names.push(NONE);
  return names.map((name) => ({ name, n: n(name) }));
}

export interface ListState {
  // nothing: no task nor pin anywhere (#tasks-empty, the screen of the list alone)
  nothing: boolean;
  // none: no task of the view, or the project is gone (#tasks-none); gone: no such project (#tasks-other)
  none: boolean;
  gone: boolean;
  // addHidden: the shelf's way to add a task gives way
  addHidden: boolean;
}

// listState is what the task list says besides its rows
export function listState(
  tasks: readonly TaskOf[],
  entries: readonly Page[],
  project: string,
  view: string,
  failed: boolean,
  shown: number,
): ListState {
  const there = taskProjects(tasks, entries);
  const nothing = !failed && there.size === 0;
  const gone = !there.has(project);
  return {
    nothing,
    none: !nothing && !failed && shown === 0,
    gone,
    addHidden: view === 'archived' || nothing || failed || gone,
  };
}

// unreadOf is how many of a task's pages are unread
export const unreadOf = (task: TaskOf): number => task.pages.filter((a) => a.news).length;

// urlEnd is the end of a task's URL, what tells one from another: its last two parts of path, or its host
export function urlEnd(url: string): string {
  try {
    const at = new URL(url);
    return at.pathname.split('/').filter(Boolean).slice(-2).join('/') || at.host;
  } catch {
    return url;
  }
}

// taskRowKey is the key of a task's row on the task list
export const taskRowKey = (project: string, name: string): string => `task:${JSON.stringify([project, name])}`;

// projectPress is what a press of a project's chip does: the list always shows one project, so the one shown pressed
// again changes nothing; the project pressed is kept for the next opening once the tasks are read, not while
// unreadable, when the projects there are not known
export const projectPress = (name: string, shown: string, failed: boolean): { keep: boolean; show: boolean } => ({
  keep: !failed,
  show: name !== shown,
});
