import type { Client } from '@/lib/api-client';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';
import type { Tasks, TaskWritten } from '@/types/task';

const TASKS_API = '/api/tasks';

// The tasks as read: their error, which only the task screens show, and the text, to tell a change
export interface TasksRead {
  error: string;
  tasks: Tasks;
  raw: string;
}

// readTasks never fails: the list of pages goes on without the tasks, and the task list says why
export async function readTasks(client: Client, signal: AbortSignal): Promise<TasksRead> {
  try {
    const res = await client.api(TASKS_API, { signal });
    const raw = await res.text();
    return res.ok ? { error: '', tasks: JSON.parse(raw) as Tasks, raw } : { error: raw, tasks: {}, raw };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return { error: message, tasks: {}, raw: message };
  }
}

// What a write of pages and their tasks sends (PATCH /api/tasks); new and rev say what the screen saw of the task
export interface TasksChange {
  taskProject?: string;
  task?: string;
  slugs?: string[];
  category?: string;
  new?: boolean;
  rev?: number;
  [field: string]: unknown;
}

// A page a write of pages and their tasks touched, with the task it was on before, as the store found it under its
// lock (store's offTask, offCategory): '' or left out when it was on none
export interface PageWritten extends Page {
  offTask?: string;
  offCategory?: string;
}

export interface TasksAnswer {
  ok: boolean;
  status: number;
  text: string;
  // The rev of the first task written, null when refused
  rev: number | null;
  // The refusal with what to do next, for a modal that can save again
  stale: string;
  pages: PageWritten[];
  written: TaskWritten[];
}

// changeTasks sends the one write of pages and their tasks. A write that says the task is there but of no rev read is
// not sent, as JSON would drop it and the store write it unchecked. It reads nothing again: stores/tasks does
export async function changeTasks(client: Client, body: TasksChange): Promise<TasksAnswer> {
  if (body.new === false && !Number.isInteger(body.rev)) {
    return { ok: false, status: 0, text: t('ui_task_no_rev'), rev: null, stale: '', pages: [], written: [] };
  }
  const res = await client.api(TASKS_API, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  });
  const text = res.ok ? '' : await res.text();
  let written: TaskWritten[] = [];
  let pages: PageWritten[] = [];
  if (res.ok) {
    const answer = (await res.json()) as { tasks: TaskWritten[]; pages: PageWritten[] };
    written = answer.tasks;
    pages = answer.pages;
  }
  const rev = written[0]?.row.rev ?? null;
  return { ok: res.ok, status: res.status, text, rev, stale: t('ui_task_stale', text), pages, written };
}

// deleteTask sends the delete of the task name of project
export const deleteTask = (client: Client, project: string, name: string): Promise<Response> =>
  client.api(`${TASKS_API}?${new URLSearchParams({ taskProject: project, task: name }).toString()}`, {
    method: 'DELETE',
  });
