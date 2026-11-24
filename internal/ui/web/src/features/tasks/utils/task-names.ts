import { stateWord } from '@/features/tasks/utils/kin-forms';
import { t } from '@/lib/t';
import type { TaskOf } from '@/types/task';
import type { Offer } from '@/utils/name-offers';

// taskNames is the names of a project's tasks a field that names a task offers, by name
export const taskNames = (
  tasks: readonly TaskOf[],
  project: string,
  offered: (x: TaskOf) => boolean,
  lang: string,
): string[] =>
  tasks
    .filter((x) => x.project === project && offered(x))
    .map((x) => x.name)
    .sort((x, y) => x.localeCompare(y, lang));

// taskOffer is how a task is offered: its name and title, and after them what it is in, which a long title does not
// cut (a task done is under another view than the list's first, and looked for there), and its parent
export function taskOffer(tasks: readonly TaskOf[], project: string, name: string): Offer {
  const task = tasks.find((x) => x.project === project && x.name === name);
  const note = task ? t('ui_task_suggest', stateWord(task.state)) : '';
  return {
    name,
    label: [name, task?.title].filter(Boolean).join(' '),
    note: task?.parent ? `${note} ${t('ui_task_suggest', t('ui_task_parent_named', task.parent))}` : note,
  };
}
