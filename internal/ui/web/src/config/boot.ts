import type { MessageKey } from '@/types/messages.gen';
import type { State } from '@/types/task';

// What the server tells the page as it starts, the JSON of the script #boot (internal/ui's boot)
export interface Boot {
  screen: 'list' | 'tasks' | 'frame';
  lang: string;
  messages: Record<MessageKey, string>;
  // '' on the lists: not a frame
  framed: string;
  served: boolean;
  // '' where ?v= asks none
  version: string;
  states: State[];
  maxURLs: number;
}

// Screen is the screen shown: the page list, the task list, a task's page (the task list's URL with task=), or a page
// whole. The one place a task's page is told from the task list (ui.html TASKS, TASK_PAGE, TASK_LIST)
export type Screen = 'list' | 'taskList' | 'taskPage' | 'frame';

export const screenOf = (boot: Pick<Boot, 'screen'>, search: string): Screen => {
  if (boot.screen !== 'tasks') return boot.screen;
  return new URLSearchParams(search).has('task') ? 'taskPage' : 'taskList';
};

const NONE = 'the page has no boot values: reload it';

// readBoot reads the boot values the server wrote into the script #boot. They are floe's own, so none, or none that
// is JSON, is floe's mistake; their shape is not checked, as the API's answers are not.
export function readBoot(text: string | null | undefined): Boot {
  if (!text) throw new Error(NONE);
  try {
    return JSON.parse(text) as Boot;
  } catch (err) {
    throw new Error(`${NONE} (${err instanceof Error ? err.message : String(err)})`, { cause: err });
  }
}

let kept: Boot | null = null;

// setBoot keeps the boot values for getBoot, before anything is drawn
export function setBoot(boot: Boot): void {
  kept = boot;
}

// getBoot is what setBoot kept: read when called, never as a module loads
export function getBoot(): Boot {
  if (kept === null) throw new Error('getBoot is called before setBoot');
  return kept;
}
