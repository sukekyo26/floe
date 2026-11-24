import { TASKS_PATH } from '@/utils/page-urls';

// Where a page whole goes back to, as its button says it: the page list, the task list, or a task's page
export interface Back {
  href: string;
  to: 'pages' | 'tasks' | 'task';
  task: string;
}

// backOf reads the way back kept, a path and query of this origin
// LIMIT: the name is the one in the URL kept: a task gone since, or a page moved to another, still says it; the screen
// it leads to tells. Read the task if that comes to mislead
export function backOf(kept: string, origin: string): Back {
  const url = new URL(kept, origin);
  const task = url.searchParams.get('task') ?? '';
  const href = url.pathname + url.search;
  if (url.pathname !== TASKS_PATH) return { href, to: 'pages', task: '' };
  return { href, to: task ? 'task' : 'tasks', task };
}
