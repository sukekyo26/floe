import type { Page } from '@/types/page';
import type { TaskOf, Tasks } from '@/types/task';

// tasksFrom is each task of tasks.json, with the pages on it. A task is of its row alone: a page naming a task with no
// row, as a hand edit leaves, is on none here. A child, as the store's childrenOfTask says, is of a parent with a row
// of the same project and no parent itself
export function tasksFrom(entries: readonly Page[], read: Tasks): TaskOf[] {
  const tasks = new Map<string, TaskOf>();
  const key = (project: string, name: string) => JSON.stringify([project, name]);
  for (const [project, rows] of Object.entries(read)) {
    for (const [name, row] of Object.entries(rows)) {
      // A row of no rev, rev 0, as the store counts it
      tasks.set(key(project, name), { rev: 0, ...row, project, name, pages: [], archived: [], children: [] });
    }
  }
  for (const a of entries) {
    const task = a.task?.name ? tasks.get(key(a.taskProject, a.task.name)) : undefined;
    task?.[a.archived ? 'archived' : 'pages'].push(a);
  }
  for (const [project, rows] of Object.entries(read)) {
    for (const [name, { parent }] of Object.entries(rows)) {
      if (!parent || !Object.hasOwn(rows, parent) || rows[parent]?.parent) continue;
      const child = tasks.get(key(project, name));
      const of = tasks.get(key(project, parent));
      if (!child || !of) continue;
      child.parentTask = of;
      of.children.push(child);
    }
  }
  return [...tasks.values()];
}

// knownTasks is the tasks of what was read: none while tasks.json is unreadable, as they are not known
export const knownTasks = (entries: readonly Page[], taken: { error: string; tasks: Tasks }): TaskOf[] =>
  taken.error ? [] : tasksFrom(entries, taken.tasks);
