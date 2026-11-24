import { useMemo, useState } from 'react';

import { taskRanks, taskProjects } from '@/features/tasks/utils/task-list';
import { pageTask } from '@/features/tasks/utils/task-page';
import { useStore } from '@/hooks/use-store';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';
import type { TaskOf } from '@/types/task';
import { projectColor } from '@/utils/project-color';
import { tasksFrom } from '@/utils/tasks-of';

export interface TaskPageView {
  read: boolean;
  project: string;
  // task is what the page draws; head what its head draws, the task as last drawn once it is gone
  task: TaskOf | null;
  head: TaskOf | null;
  none: boolean;
  missing: boolean;
  // reads is the reads of the list ended, at each of which the state's choices are taken again
  reads: number;
  // color is the project's, as the task list paints it
  color: string;
}

export interface TaskPageDeps {
  query?: ListQuery;
  pages?: Pages;
}

// useTaskPage is what a task's page shows of the task named, as the list read is now: the task as last read while
// tasks.json is unreadable, with what the screen wrote since
export function useTaskPage(name: string, { query = listQuery, pages = pagePages }: TaskPageDeps = {}): TaskPageView {
  const { project } = useStore(query.query);
  const { read, entries, tasks: taken } = useStore(pages.state);
  const reads = useStore(pages.ended);
  const failed = Boolean(taken.error);
  // Unreadable, the tasks are those the screen wrote since (pages.takeTasks), on none else
  const all = useMemo(() => tasksFrom(entries, taken.tasks), [entries, taken.tasks]);
  const found = all.find((x) => x.project === project && x.name === name);
  const [last, setLast] = useState<TaskOf | null>(null);
  if (!failed && (found ?? null) !== last) setLast(found ?? null);
  const rows = taken.tasks[project];
  const shown = useMemo(() => pageTask({ found, rows, failed, last }), [found, rows, failed, last]);
  const [head, setHead] = useState<TaskOf | null>(null);
  if (shown.task && shown.task !== head) setHead(shown.task);
  const color = useMemo(
    () => projectColor(project, taskRanks(entries, taskProjects(failed ? [] : all, entries))),
    [project, entries, all, failed],
  );
  return {
    read,
    project,
    task: shown.task,
    head: shown.task ?? head,
    none: shown.none,
    // Nothing is said before the first read
    missing: read && shown.missing,
    reads,
    color,
  };
}
