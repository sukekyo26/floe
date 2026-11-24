import { kinButton, partButton } from '@/components/focus-places';
import { client as pageClient, type Client } from '@/lib/api-client';
import { t } from '@/lib/t';
import { deleteTask } from '@/lib/tasks';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { pressOnce, type PressOnce } from '@/stores/press-once';
import { taskWrites, type TaskWrites } from '@/stores/tasks';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import { undoToast as pageUndo, type UndoToast } from '@/stores/undo-toast';
import type { TaskOf } from '@/types/task';

// A change to a parent taken back: the parent before written again over the one written now, of the rev that write
// answered. up: the task written is the page's own (its parent's row is where the focus goes back to)
export interface ParentBack {
  project: string;
  name: string;
  parent: string;
  rev: number | null;
  up: boolean;
}

// Where the focus goes on a task's page: a row of the family card (its link or its button that takes it off), the
// button of a part of the card, and whether the card has it
export interface FamilyPlaces {
  kin: (task: string, unlink: boolean) => boolean;
  part: (up: boolean) => boolean;
  inFamily: () => boolean;
}

const places0: FamilyPlaces = {
  kin: kinButton,
  part: partButton,
  inFamily: () => document.activeElement?.closest('#task-family') !== null,
};

interface Deps {
  writes?: TaskWrites;
  pages?: Pages;
  notice?: Notice;
  toasts?: Toasts;
  undoToast?: UndoToast;
  client?: Client;
  places?: FamilyPlaces;
  press?: PressOnce;
}

export interface TaskPageActs {
  // changeState writes the task's state of the rev drawn and tells answered the rev written, or null refused, before
  // the list is read again (but on a refusal as stale, which taskWrites reads), then says the refusal or takes it away
  changeState: (
    project: string,
    name: string,
    rev: number,
    state: string,
    answered: (written: number | null) => void,
  ) => Promise<void>;
  // archive archives the task or gives it back as done, of the rev read: a second press of a double one (detail > 1)
  // and a press for what is being written write nothing
  // LIMIT: Enter held on the button turns the task back and forth at each repeat. Read the key's repeat if that comes to
  // trouble someone
  archive: (detail: number, task: TaskOf, archived: boolean) => Promise<void>;
  // unlink takes child off its parent, of the rev read, with the way back; pressed again while it goes, nothing more.
  // From the keys the focus goes to the way back, else, once read again, to the part's button unless the card has it
  // LIMIT: taken off again from the keys before the read of one by the mouse, the focus goes to the part's button, off
  // the second way back. Give it only while nothing has the focus if that comes to trouble someone
  unlink: (child: TaskOf, parent: string, page: string, fromKeys: boolean) => Promise<void>;
  // forget removes the row of a task of no page: '' once removed (gone already too), else the reason
  forget: (project: string, name: string) => Promise<string>;
  // parentBack shows the toast of a change to a parent with its way back
  parentBack: (said: string, back: ParentBack, focus?: boolean) => void;
}

// createTaskPageActs is what a task's page writes
export function createTaskPageActs({
  writes = taskWrites,
  pages = pagePages,
  notice = pageNotice,
  toasts = pageToasts,
  undoToast = pageUndo,
  client = pageClient,
  places = places0,
  press = pressOnce,
}: Deps = {}): TaskPageActs {
  // LIMIT: a write of the screen's own since, its state say, refuses it too. Take back the screen's own writes too if
  // that comes to trouble someone
  const parentBack = (said: string, back: ParentBack, focus = false) => {
    undoToast(
      said,
      {
        undo: async () => {
          const res = await writes.writeTask(back.project, back.name, {
            new: false,
            ...(back.rev === null ? {} : { rev: back.rev }),
            parent: back.parent,
            parentNew: false,
          });
          return { ok: res.ok, text: res.text };
        },
        done: () => {
          places.kin(back.up ? back.parent : back.name, true);
        },
        away: () => {
          places.part(back.up);
        },
      },
      { focus },
    );
  };

  return {
    changeState: async (project, name, rev, state, answered) => {
      let why: string | null = null;
      let again = true;
      try {
        const res = await writes.writeTask(project, name, { new: false, rev, state });
        if (!res.ok) {
          why = res.text;
          again = res.status !== 409;
        }
        answered(res.ok ? res.rev : null);
      } catch (err) {
        why = err instanceof Error ? err.message : String(err);
        answered(null);
      }
      // Before the words: a read does not take the refusal away
      if (again) await pages.load();
      notice.say(why === null ? '' : t('ui_state_failed', why));
    },
    archive: async (detail, task, archived) => {
      if (detail > 1) return;
      const state = archived ? 'archived' : 'done';
      try {
        const res = await press(`archive task ${task.project}\n${task.name}`, state, () =>
          writes.writeTask(task.project, task.name, {
            new: false,
            rev: pages.readRev(task.project, task.name),
            state,
          }),
        );
        if (!res) return;
        if (!res.ok) {
          if (res.status !== 409) await pages.load();
          notice.say(t('ui_save_failed', res.text));
          return;
        }
        toasts.toast(t(archived ? 'ui_task_arc_done' : 'ui_task_unarc_done', task.name));
        await pages.load();
      } catch (err) {
        await pages.load();
        notice.say(t('ui_save_failed', err instanceof Error ? err.message : String(err)));
      }
    },
    unlink: async (child, parent, page, fromKeys) => {
      try {
        const res = await press(`unlink ${child.project}\n${child.name}`, '', () =>
          writes.writeTask(child.project, child.name, {
            new: false,
            rev: pages.readRev(child.project, child.name),
            parent: '',
          }),
        );
        if (!res) return;
        if (!res.ok) {
          notice.say(t('ui_save_failed', res.text));
          return;
        }
        const up = child.name === page;
        parentBack(
          t('ui_task_unlinked', child.name, parent),
          { project: child.project, name: child.name, parent, rev: res.rev, up },
          fromKeys,
        );
        await pages.load();
        if (!fromKeys && !places.inFamily()) places.part(up);
      } catch (err) {
        notice.say(t('ui_save_failed', err instanceof Error ? err.message : String(err)));
      }
    },
    forget: async (project, name) => {
      try {
        const res = await deleteTask(client, project, name);
        if (!res.ok && res.status !== 404) return await res.text();
        return '';
      } catch (err) {
        return err instanceof Error ? err.message : String(err);
      }
    },
    parentBack,
  };
}

export const taskPageActs = createTaskPageActs();
