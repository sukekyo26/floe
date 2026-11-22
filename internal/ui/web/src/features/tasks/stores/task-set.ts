import { getBoot, type Boot } from '@/config/boot';
import { setBody, setSaid } from '@/features/tasks/utils/task-set';
import type { TasksAnswer } from '@/lib/tasks';
import { pageActions, type PageActions, type TakenOff, type TaskOutFocus } from '@/stores/page-actions';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { taskWrites, type TaskWrites } from '@/stores/tasks';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { FocusBack } from '@/types/focus';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';

// What Set the task is opened for: the page, and the way back to the menu it came from
export interface TaskSetOpen {
  page: Page;
  back: FocusBack;
}

// openTaskSet opens Set the task over page (show: the app's state of the modal). A page whole reads the tasks first,
// as its load reads none
export async function openTaskSet(
  page: Page,
  back: FocusBack,
  show: (open: TaskSetOpen) => void,
  { pages = pagePages, boot = getBoot }: { pages?: Pages; boot?: () => Boot } = {},
): Promise<void> {
  if (boot().screen === 'frame') await pages.rereadTasks();
  show({ page, back });
}

export interface TaskSetActs {
  // put puts page on the task name's category in one write, as the modal shows the task (shown), and says what came
  // of it once written; refused, it says nothing
  put: (
    page: Page,
    name: string,
    call: { shown: TaskOf | undefined; title: string; category: string; opened: string },
  ) => Promise<TasksAnswer>;
  // takeOff takes page off its task for the modal's way off (prepare: where the focus goes after, taken first): the
  // reason it was refused, a write that could not be sent too, or the toast its way back is in
  takeOff: (page: Page, prepare: (slug: string) => TaskOutFocus) => Promise<TakenOff>;
}

export function createTaskSetActs({
  writes = taskWrites,
  toasts = pageToasts,
  boot = getBoot,
  actions = pageActions,
}: {
  writes?: TaskWrites;
  toasts?: Toasts;
  boot?: () => Boot;
  actions?: Pick<PageActions, 'takeOff'>;
} = {}): TaskSetActs {
  return {
    takeOff: async (page, prepare) => {
      try {
        return await actions.takeOff(page, prepare, { focus: false });
      } catch (err) {
        return { reason: err instanceof Error ? err.message : String(err) };
      }
    },
    put: async (page, name, { shown, title, category, opened }) => {
      const res = await writes.putPageOn(page, name, setBody(shown, title), category.trim());
      if (!res.ok) return res;
      // The state it comes to be under stays what it was: said, as it may not be the list the person has in mind
      const state = shown?.state ?? boot().states.find((s) => s.new)?.name ?? '';
      const said = setSaid(page, name, opened, res, state);
      if ('text' in said) toasts.toast(said.text);
      else toasts.toast('', { ms: 8000, button: { label: said.link, href: said.href } });
      return res;
    },
  };
}

export const taskSetActs = createTaskSetActs();
