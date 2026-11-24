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

// A form of the modal of a task's family: what it says, the tasks it offers, what a name typed is to it, and what
// saving sends. What saving does besides is the store's (kin-acts)
export interface KinForm {
  heading: (task: KinTask) => string;
  target: (task: KinTask, ask: boolean) => string;
  offered: (task: KinTask, x: TaskOf) => boolean;
  there: (task: KinTask, name: string, x: TaskOf | undefined) => There | null;
  body: (task: KinTask, name: string, x: TaskOf | undefined, title: string) => TasksChange;
}

export type KinFormName = 'add' | 'child' | 'parent';

// stateWord is a state's name as the screen says it
export const stateWord = (state: string): string => t(`ui_state_${state}` as MessageKey);

// named is a task as its name and title say it
const named = (x: Pick<Partial<TaskOf>, 'name' | 'title'>): string => [x.name, x.title].filter(Boolean).join(' ');
// isOwnChild: x is a child of the task of the page already
const isOwnChild = (task: KinTask, x: TaskOf): boolean => x.parentTask?.name === task.name;
// mayParent: x has no parent of its own, and is not the task it is for
const mayParent = (task: KinTask, x: TaskOf): boolean => !x.parent && x.name !== task.name;

export const KIN_FORMS: Record<KinFormName, KinForm> = {
  // A child of the task of the page: a task named comes to be one with what it has, out of another parent's, and any
  // other is made. What the store's checkParent would refuse is not offered; pressed, the store refuses it
  child: {
    heading: () => t('ui_child_add_named'),
    target: (task) => t('ui_task_child_of', named(task)),
    offered: (task, x) => x.children.length === 0 && x.name !== task.name && !isOwnChild(task, x),
    there: (task, _name, x) => {
      if (!x) return null;
      if (isOwnChild(task, x)) return { button: 'ui_child_take_button', act: 'none', note: '' };
      const said = x.parent ? t('ui_task_child_move', named(x), x.parent) : t('ui_task_child_take', named(x));
      return { button: 'ui_child_take_button', act: 'write', note: KIN_FORMS.child.offered(task, x) ? said : '' };
    },
    // LIMIT: this task's row gone meanwhile, its pages left, it is there still: refused as no parent (400), not read
    // again, as a parent not known
    body: (task, name, x, title) => ({
      taskProject: task.project,
      task: name,
      new: !x,
      ...(x ? { rev: x.rev } : { title }),
      parent: task.name ?? '',
      parentNew: false,
    }),
  },
  // The parent of the task of the page: a task named comes to be it, the one before said; any other is made with it, in
  // the same write (parentTitle)
  parent: {
    heading: (task) => t(task.parent ? 'ui_up_change_named' : 'ui_up_add_named'),
    target: (task) => t('ui_task_parent_of', named(task)),
    offered: mayParent,
    there: (task, name, x) => {
      // The parent as written, one that makes no family too, named again is no change
      const written = task.parent?.trim();
      if (written && name === written) return { button: 'ui_up_take_button', act: 'none', note: '' };
      if (!x) return null;
      const said =
        task.parent && task.parent !== x.name
          ? t('ui_task_parent_move', named(x), task.parent)
          : t('ui_task_parent_take', named(x));
      return { button: 'ui_up_take_button', act: 'write', note: mayParent(task, x) ? said : '' };
    },
    // LIMIT: whether the one named has come to have a parent or children meanwhile is not said: the store refuses it as
    // it is now (400), and the words say so, with nothing read again
    body: (task, name, x, title) => ({
      taskProject: task.project,
      task: task.name ?? '',
      new: false,
      ...(task.rev === undefined ? {} : { rev: task.rev }),
      parent: name,
      parentNew: !x,
      ...(x ? {} : { parentTitle: title }),
    }),
  },
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

// askedProjects is what the modal adding a task asks the project of, with no task nor pin anywhere: no project, then
// the pages' projects in the order of their colours; the one chosen is the list's if a page has it. A project no page
// has is for floe task --project
// LIMIT: the first task made, the other projects are not on the screen to make one in: a page's "Set task" makes one in
// its project and floe task --project in any. Ask the project every time if that comes to trouble someone
export function askedProjects(entries: readonly Page[], project: string): { names: string[]; chosen: string } {
  const names = [...new Set(entries.map((a) => a.taskProject))].filter((name) => name !== NONE);
  const ranks = projectOrder(entries, names);
  names.sort((x, y) => (ranks.get(x) ?? 0) - (ranks.get(y) ?? 0));
  return { names: [NONE, ...names], chosen: names.includes(project) ? project : NONE };
}
