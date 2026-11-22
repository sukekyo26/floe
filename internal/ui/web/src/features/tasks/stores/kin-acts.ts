import { getBoot, type Boot } from '@/config/boot';
import { projectKept, type ProjectKept } from '@/features/tasks/stores/project-kept';
import { KIN_FORMS, type KinFormName, type KinTask } from '@/features/tasks/utils/kin-forms';
import { taskRowKey } from '@/features/tasks/utils/task-list';
import { t } from '@/lib/t';
import type { TasksAnswer } from '@/lib/tasks';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { taskWrites, type TaskWrites } from '@/stores/tasks';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { TaskOf } from '@/types/task';
import { taskHref } from '@/utils/page-urls';

export interface KinActs {
  // write sends what the form saves, as the modal showed the task (new, rev); refused as stale (409), the tasks are read
  // again (taskWrites)
  write: (form: KinFormName, task: KinTask, name: string, x: TaskOf | undefined, title: string) => Promise<TasksAnswer>;
  // done tells what was written, once written, before the list is read again
  done: (form: KinFormName, task: KinTask, name: string) => void;
  // back is the row the focus goes to as the modal closes saved (data-fresh), null for none
  back: (form: KinFormName, task: KinTask, name: string) => string | null;
}

interface Deps {
  writes?: TaskWrites;
  toasts?: Toasts;
  query?: ListQuery;
  kept?: ProjectKept;
  boot?: () => Boot;
}

// createKinActs is what the modal of a task's family writes and does once written (ui.html KIN_FORMS' write, done and
// back)
export function createKinActs({
  writes = taskWrites,
  toasts = pageToasts,
  query = listQuery,
  kept = projectKept,
  boot = getBoot,
}: Deps = {}): KinActs {
  const link = (words: string, project: string, name: string) => {
    toasts.toast('', { ms: 8000, button: { label: words, href: taskHref({ project, name }) } });
  };
  return {
    write: (form, task, name, x, title) => writes.writeTasks(KIN_FORMS[form].body(task, name, x, title)),
    done: (_form, task, name) => {
      // The list goes where the row went, drawn by the load that follows: the write's answer is in the tasks read
      // already (taskWrites), so the state it goes to has the row as it is drawn
      link(t('ui_task_made', name), task.project, name);
      kept.keep(task.project);
      query.filter({ project: task.project, view: boot().states.find((s) => s.new)?.name ?? '' });
    },
    back: (_form, task, name) => taskRowKey(task.project, name),
  };
}

export const kinActs = createKinActs();
