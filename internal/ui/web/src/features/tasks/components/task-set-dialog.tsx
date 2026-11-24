import { useLayoutEffect, useRef, useState, type MouseEvent } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, HINT, TEXT } from '@/components/fields';
import { focusOn, prepareTaskOut } from '@/components/focus-places';
import { TaskSetIcon } from '@/components/icons';
import { useNameSuggest } from '@/components/name-suggest';
import { getBoot } from '@/config/boot';
import { useSeenTask } from '@/features/tasks/hooks/use-seen-task';
import { moveAsk as ask0, type MoveAsk } from '@/features/tasks/stores/move-ask';
import { taskSetActs as acts0, type TaskSetActs, type TaskSetOpen } from '@/features/tasks/stores/task-set';
import { taskNames, taskOffer } from '@/features/tasks/utils/task-names';
import { useHold } from '@/hooks/use-hold';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import type { TakenOff } from '@/stores/page-actions';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { nameOffers } from '@/utils/name-offers';
import { knownTasks } from '@/utils/tasks-of';

export interface TaskSetDialogProps {
  readonly open: TaskSetOpen | null;
  readonly onClosed: () => void;
  readonly pages?: Pages;
  readonly acts?: TaskSetActs;
  readonly ask?: MoveAsk;
}

// TaskSetDialog puts a page on a task and a category of it: the task's own fields are its page's to write (the edit),
// this writes the page's category, and a new name's title. The task named is shown as found when it opens, as the name
// changes and as a save is refused (useSeenTask): what changed since is refused, and said, with the modal open. A page
// on another task is asked about first (MoveDialog). Its way off the task takes the page off at once, held, the way
// back in a toast. Closed, the focus goes back to the menu
export function TaskSetDialog({ open, onClosed, pages = pagePages, acts = acts0, ask = ask0 }: TaskSetDialogProps) {
  const hold = useHold();
  const { entries, tasks: taken } = useStore(pages.state);
  // The last opened, which the modal shows and goes back from as it closes: open is null by then
  const [last, setLast] = useState<TaskSetOpen | null>(null);
  const [name, setName] = useState('');
  const [title, setTitle] = useState('');
  const [category, setCategory] = useState('');
  const [opened, setOpened] = useState('');
  const [error, setError] = useState('');
  const nameField = useRef<HTMLInputElement>(null);
  const titleField = useRef<HTMLInputElement>(null);
  // The toast whose way back takes the focus as it closes: taken off from the keys
  const toToast = useRef<number | null>(null);
  const failed = Boolean(taken.error);
  // The fields as they open: the page's task and category, none of what was typed before
  if (open !== null && open !== last) {
    setLast(open);
    const now = open.page.task?.name ?? '';
    setName(now);
    setTitle('');
    setCategory(open.page.task?.category ?? '');
    setOpened(now);
    setError(failed ? t('ui_tasks_failed', taken.error) : '');
  }
  const page = open?.page;
  const project = page?.taskProject ?? '';
  const tasks = knownTasks(entries, taken);
  const shown = useSeenTask(open, project, name, tasks);
  // The title is asked of a new task alone: hidden, it is not required, nor marked blank
  const titleShown = shown.seen === undefined;
  useLayoutEffect(() => {
    if (open === null) return;
    for (const input of [nameField.current, titleField.current]) input?.setCustomValidity('');
  }, [open]);
  useLayoutEffect(() => {
    if (!titleShown) titleField.current?.setCustomValidity('');
  }, [titleShown]);

  const lang = getBoot().lang;
  const suggestTask = useNameSuggest({
    id: 'set-task',
    input: nameField,
    offers: (written, typed) =>
      nameOffers(
        taskNames(tasks, project, () => true, lang),
        written,
        typed,
        {
          detail: (n) => taskOffer(tasks, project, n),
          fresh: (n) => ({ name: n, label: t('ui_task_new', n) }),
        },
      ),
    put: (n) => {
      nameField.current?.setCustomValidity('');
      setName(n);
    },
    // The modal hands the first field the focus as it opens: not over the fields under it
    onFocus: false,
  });
  const suggestCategory = useNameSuggest({
    id: 'set-category',
    // The categories of the task named, as read now, in their order
    offers: (written, typed) =>
      nameOffers(tasks.find((x) => x.project === project && x.name === name.trim())?.categories ?? [], written, typed),
    put: setCategory,
  });

  // Whether the form may be sent: tasks.json read, and no field required that has spaces alone
  const ready = (): boolean => {
    if (failed) {
      setError(t('ui_tasks_failed', taken.error));
      return false;
    }
    const blank = [nameField.current, titleShown ? titleField.current : null].find(
      (input) => input !== null && !input.value.trim(),
    );
    if (!blank) return true;
    blank.setCustomValidity(t('ui_field_blank'));
    blank.reportValidity();
    return false;
  };

  const save = async () => {
    if (!page || !ready()) return;
    // Asked once the form may be sent, not of what would be refused; No leaves the modal as it was
    if (!(await ask.ask(page, name))) {
      focusOn(document.getElementById('set-submit'));
      return;
    }
    try {
      const ran = await hold.whileHeld(async () => {
        const res = await acts.put(page, name, { shown: shown.seen, title, category, opened });
        // Refused as stale: whether the name is of a task there, as read again, a title asked of a new one alone
        if (res.status === 409) {
          shown.renew();
          throw new Error(res.stale);
        }
        if (!res.ok) throw new Error(res.text);
        await pages.load();
      });
      if (!ran) return;
    } catch (err) {
      setError(t('ui_save_failed', err instanceof Error ? err.message : String(err)));
      return;
    }
    onClosed();
  };

  // The modal's way off the task: closed once off, the focus back on the menu, and from the keys then on the way back
  const takeOut = async (e: MouseEvent<HTMLButtonElement>) => {
    if (!open || !page) return;
    const keys = e.detail === 0;
    const got: { off: TakenOff } = { off: { reason: '' } };
    const ran = await hold.whileHeld(async () => {
      got.off = await acts.takeOff(page, (slug) => prepareTaskOut(slug, open.back));
      if (!got.off.reason) await pages.load();
    });
    if (!ran) return;
    if (got.off.reason) {
      setError(t('ui_save_failed', got.off.reason));
      return;
    }
    toToast.current = keys ? (got.off.toast ?? null) : null;
    onClosed();
  };

  const clearMark = (e: { currentTarget: HTMLInputElement }) => {
    e.currentTarget.setCustomValidity('');
  };
  return (
    <Dialog
      id="set-dialog"
      open={open !== null}
      onClosed={onClosed}
      hold={hold}
      labelledBy="set-title"
      afterClose={() => {
        const toast = toToast.current;
        toToast.current = null;
        const back = last?.back() ?? false;
        if (toast === null) return back;
        return focusOn(document.querySelector(`[data-toast="${String(toast)}"] button`));
      }}
    >
      <form
        id="set"
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <DialogHead
          icon={<TaskSetIcon />}
          titleId="set-title"
          title={t('ui_task_set')}
          lead={page?.title ?? page?.slug ?? ''}
          leadId="set-target"
        />
        <label data-testid="field" className={FIELD}>
          {t('ui_task_name')}
          <input
            id="set-task"
            data-testid="text"
            className={TEXT}
            required
            autoComplete="off"
            spellCheck={false}
            placeholder={t('ui_task_id_example')}
            data-autofocus=""
            value={name}
            {...suggestTask.field}
            onInput={(e) => {
              clearMark(e);
              suggestTask.field.onInput(e);
            }}
            onChange={(e) => {
              setName(e.currentTarget.value);
            }}
          />
        </label>
        {suggestTask.box}
        <label id="set-name-title-field" data-testid="field" className={FIELD} hidden={!titleShown}>
          {t('ui_task_title')}
          <input
            ref={titleField}
            id="set-task-title"
            data-testid="text"
            className={TEXT}
            required={titleShown && !failed}
            autoComplete="off"
            value={title}
            onInput={clearMark}
            onChange={(e) => {
              setTitle(e.currentTarget.value);
            }}
          />
        </label>
        <label data-testid="field" className={FIELD}>
          <span>
            {t('ui_task_category')} <small className={HINT}>{t('ui_optional')}</small>
          </span>
          <input
            id="set-category"
            data-testid="text"
            className={TEXT}
            autoComplete="off"
            placeholder={t('ui_task_category_example')}
            value={category}
            {...suggestCategory.field}
            onInput={(e) => {
              clearMark(e);
              suggestCategory.field.onInput(e);
            }}
            onChange={(e) => {
              setCategory(e.currentTarget.value);
            }}
          />
        </label>
        {suggestCategory.box}
        <p
          id="set-error"
          hidden={!error}
          className="m-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
        >
          {error}
        </p>
        <DialogActions>
          <span className="me-auto">
            <Button id="set-out" hidden={!page?.task?.name} onClick={(e) => void takeOut(e)}>
              {t('ui_task_out')}
            </Button>
          </span>
          <DialogClose words="ui_cancel" />
          <Button id="set-submit" kind="primary" type="submit">
            {t('ui_save')}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
