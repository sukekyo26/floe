import { getBoot, type Boot } from '@/config/boot';
import { projectKept, type ProjectKept } from '@/features/tasks/stores/project-kept';
import { taskPageActs, type TaskPageActs } from '@/features/tasks/stores/task-page-acts';
import { KIN_FORMS, type KinFormName, type KinTask } from '@/features/tasks/utils/kin-forms';
import { taskRowKey } from '@/features/tasks/utils/task-list';
import { t } from '@/lib/t';
import type { TasksAnswer } from '@/lib/tasks';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { taskWrites, type TaskWrites } from '@/stores/tasks';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { TaskOf } from '@/types/task';
import { taskHref } from '@/utils/page-urls';

// Where the focus goes as the modal closes saved: the row made (marked fresh), or the button of a part of the family
// card, the parent's (up) or the children's
export type KinBack = { row: string } | { part: 'up' | 'kids' };

export interface KinActs {
  // write sends what the form saves, as the modal showed the task (new, rev); refused as stale (409), the tasks are read
  // again (taskWrites)
  write: (form: KinFormName, task: KinTask, name: string, x: TaskOf | undefined, title: string) => Promise<TasksAnswer>;
  // done tells what was written, once written, before the list is read again: x the task named as the modal showed
  // it, rev the rev the write answered
  done: (form: KinFormName, task: KinTask, name: string, x: TaskOf | undefined, rev: number | null) => void;
  // back is where the focus goes as the modal closes saved, or with nothing to write
  back: (form: KinFormName, task: KinTask, name: string) => KinBack;
}

interface Deps {
  writes?: TaskWrites;
  toasts?: Toasts;
  query?: ListQuery;
  kept?: ProjectKept;
  boot?: () => Boot;
  page?: Pick<TaskPageActs, 'parentBack'>;
}

// createKinActs is what the modal of a task's family writes and does once written
export function createKinActs({
  writes = taskWrites,
  toasts = pageToasts,
  query = listQuery,
  kept = projectKept,
  boot = getBoot,
  page = taskPageActs,
}: Deps = {}): KinActs {
  const link = (words: string, project: string, name: string) => {
    toasts.toast('', { ms: 8000, button: { label: words, href: taskHref({ project, name }) } });
  };
  return {
    write: (form, task, name, x, title) => writes.writeTasks(KIN_FORMS[form].body(task, name, x, title)),
    done: (form, task, name, x, rev) => {
      if (form === 'child') {
        link(t(x ? 'ui_task_child_took' : 'ui_task_child_made', name), task.project, name);
        return;
      }
      if (form === 'parent') {
        // Changed, the way back is offered while the parent before makes a family: one written by hand that makes none
        // the store would refuse
        if (!task.parent) link(t('ui_task_parent_put', name), task.project, name);
        else if (task.parentTask?.name === task.parent)
          page.parentBack(t('ui_task_parent_moved', task.parent, name), {
            project: task.project,
            name: task.name ?? '',
            parent: task.parent,
            rev,
            up: true,
          });
        else toasts.toast(t('ui_task_parent_moved', task.parent, name), { ms: 8000 });
        return;
      }
      // The list goes where the row went, drawn by the load that follows: the write's answer is in the tasks read
      // already (taskWrites), so the state it goes to has the row as it is drawn
      link(t('ui_task_made', name), task.project, name);
      kept.keep(task.project);
      query.filter({ project: task.project, view: boot().states.find((s) => s.new)?.name ?? '' });
    },
    back: (form, task, name) =>
      form === 'add' ? { row: taskRowKey(task.project, name) } : { part: form === 'parent' ? 'up' : 'kids' },
  };
}

export const kinActs = createKinActs();
