import { getBoot, type Boot } from '@/config/boot';
import { client as pageClient, type Client } from '@/lib/api-client';
import { t } from '@/lib/t';
import { changeTasks, type TasksAnswer, type TasksChange } from '@/lib/tasks';
import { pages as pagePages, type Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';
import { tasksFrom } from '@/utils/tasks-of';

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

  const tasksOf = (): TaskOf[] => {
    const { entries, tasks } = pages.state.get();
    return tasksFrom(entries, tasks.tasks);
  };

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
