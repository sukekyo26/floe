import { projectWord, t } from '@/lib/t';
import type { TasksChange } from '@/lib/tasks';
import type { MessageKey } from '@/types/messages.gen';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';
import { NONE } from '@/utils/list-view';
import { projectOrder } from '@/utils/project-order';

// The task a modal of a task's family is for: one read, or, adding a task, its project alone
export type KinTask = Pick<TaskOf, 'project'> & Partial<TaskOf>;

// What a name typed is to the modal, once it is no new task: what the button says, what pressing does (open the task's
// page, write nothing, or write) and the words that say so
export interface There {
  button: MessageKey;
  act: 'open' | 'none' | 'write';
  note: string;
}

// A form of the modal of a task's family (ui.html KIN_FORMS): what it says, the tasks it offers, what a name typed is
// to it, and what saving sends. What saving does besides is the store's (kin-acts)
export interface KinForm {
  heading: (task: KinTask) => string;
  target: (task: KinTask, ask: boolean) => string;
  offered: (task: KinTask, x: TaskOf) => boolean;
  there: (task: KinTask, name: string, x: TaskOf | undefined) => There | null;
  body: (task: KinTask, name: string, x: TaskOf | undefined, title: string) => TasksChange;
}

export type KinFormName = 'add';

// stateWord is a state's name as the screen says it
export const stateWord = (state: string): string => t(`ui_state_${state}` as MessageKey);

// named is a task as its name and title say it
const named = (x: TaskOf): string => [x.name, x.title].filter(Boolean).join(' ');

export const KIN_FORMS: Record<KinFormName, KinForm> = {
  // A new task, from the task list. A task named is there already, which is said, and pressing opens it with nothing
  // written
  add: {
    heading: () => t('ui_task_add'),
    target: (task, ask) => (ask ? '' : t('ui_task_add_project', projectWord(task.project))),
    offered: () => false,
    there: (_task, _name, x) =>
      x ? { button: 'ui_task_open_button', act: 'open', note: t('ui_task_there', named(x), stateWord(x.state)) } : null,
    body: (task, name, _x, title) => ({ taskProject: task.project, task: name, new: true, title }),
  },
};

// askedProjects is what the modal adding a task asks the project of, with no task nor pin anywhere (ui.html
// openTaskAdd): no project, then the pages' projects in the order of their colours; the one chosen is the list's if a
// page has it. A project no page has is for floe task --project
// LIMIT: the first task made, the other projects are not on the screen to make one in: a page's "Set task" makes one in
// its project and floe task --project in any. Ask the project every time if that comes to trouble someone
export function askedProjects(entries: readonly Page[], project: string): { names: string[]; chosen: string } {
  const names = [...new Set(entries.map((a) => a.taskProject))].filter((name) => name !== NONE);
  const ranks = projectOrder(entries, names);
  names.sort((x, y) => (ranks.get(x) ?? 0) - (ranks.get(y) ?? 0));
  return { names: [NONE, ...names], chosen: names.includes(project) ? project : NONE };
}
