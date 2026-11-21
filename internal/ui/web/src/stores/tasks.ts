import { getBoot, type Boot } from '@/config/boot';
import { client as pageClient, type Client } from '@/lib/api-client';
import { t } from '@/lib/t';
import { changeTasks, type TasksAnswer, type TasksChange } from '@/lib/tasks';
import { pages as pagePages, type Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { TaskRow } from '@/types/task';

// A task of tasks.json with the pages on it, its parent and children (one level, as the store's childrenOfTask says)
export interface TaskOf extends TaskRow {
  rev: number;
  project: string;
  name: string;
  pages: Page[];
  archived: Page[];
  children: TaskOf[];
  parentTask?: TaskOf;
}

export interface TaskWrites {
  // writeTasks is the one write of pages and their tasks, from the screen, a modal, the state's choices or the archive
  // button. Refused as stale (409), the tasks are read again, a frame's too, whose load reads none; written, every task
  // the store answers goes in the tasks read
  writeTasks: (body: TasksChange) => Promise<TasksAnswer>;
  // writeTask writes the task (project, name) alone
  writeTask: (project: string, name: string, body: TasksChange) => Promise<TasksAnswer>;
  // putPageOn puts a page on a category of a task in one write, with what the screen saw of the task (body)
  putPageOn: (page: Pick<Page, 'slug'>, task: string, body: TasksChange, category: string) => Promise<TasksAnswer>;
  // changePageTask writes the page's own task alone (its pin, or off its task and back): '' or the reason
  changePageTask: (slug: string, body: TasksChange) => Promise<string>;
  // tasksOf is each task of tasks.json, with the pages on it. A task is of its row alone: a page naming a task with no
  // row, as a hand edit leaves, is on none here
  tasksOf: () => TaskOf[];
}

interface Deps {
  client?: Client;
  pages?: Pages;
  boot?: () => Boot;
}

export function createTaskWrites({ client = pageClient, pages = pagePages, boot = getBoot }: Deps = {}): TaskWrites {
  async function writeTasks(body: TasksChange) {
    const answer = await changeTasks(client, body);
    if (answer.status === 409) {
      if (boot().screen === 'frame') await pages.rereadTasks();
      await pages.load();
    }
    if (answer.ok) pages.takeTasks(answer.written);
    return answer;
  }

  function tasksOf(): TaskOf[] {
    const { entries, tasks: read } = pages.state.get();
    const tasks = new Map<string, TaskOf>();
    const key = (project: string, name: string) => JSON.stringify([project, name]);
    for (const [project, rows] of Object.entries(read.tasks)) {
      for (const [name, row] of Object.entries(rows)) {
        // A row of no rev, rev 0, as the store counts it
        tasks.set(key(project, name), { rev: 0, ...row, project, name, pages: [], archived: [], children: [] });
      }
    }
    for (const a of entries) {
      const task = a.task?.name ? tasks.get(key(a.taskProject, a.task.name)) : undefined;
      task?.[a.archived ? 'archived' : 'pages'].push(a);
    }
    for (const [project, rows] of Object.entries(read.tasks)) {
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

  return {
    writeTasks,
    writeTask: (project, name, body) => writeTasks({ taskProject: project, task: name, ...body }),
    putPageOn: (page, task, body, category) => writeTasks({ slugs: [page.slug], task, category, ...body }),
    changePageTask: async (slug, body) => {
      const answer = await writeTasks({ slugs: [slug], ...body });
      return answer.ok ? '' : t('ui_http_error', answer.text, answer.status);
    },
    tasksOf,
  };
}

export const taskWrites = createTaskWrites();
