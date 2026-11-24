import { useLayoutEffect, useRef, useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, TEXT } from '@/components/fields';
import { focusRow, partButton } from '@/components/focus-places';
import { PlusIcon } from '@/components/icons';
import { useNameSuggest } from '@/components/name-suggest';
import { getBoot } from '@/config/boot';
import { freshTask as fresh0, type FreshTask } from '@/features/tasks/stores/fresh-task';
import { kinActs as acts0, type KinActs, type KinBack } from '@/features/tasks/stores/kin-acts';
import { askedProjects, KIN_FORMS, type KinFormName, type KinTask } from '@/features/tasks/utils/kin-forms';
import { taskNames, taskOffer } from '@/features/tasks/utils/task-names';
import { useSeenTask } from '@/features/tasks/hooks/use-seen-task';
import { useHold } from '@/hooks/use-hold';
import { useStore } from '@/hooks/use-store';
import { goTo } from '@/lib/platform';
import { projectWord, t } from '@/lib/t';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { nameOffers } from '@/utils/name-offers';
import { taskHref } from '@/utils/page-urls';
import { knownTasks } from '@/utils/tasks-of';

// What opens the modal: its form, the task it is for and whether it asks the project (no task nor pin anywhere)
export interface KinOpen {
  form: KinFormName;
  task: KinTask;
  ask: boolean;
}

export interface KinDialogProps {
  readonly open: KinOpen | null;
  readonly onClosed: () => void;
  readonly pages?: Pages;
  readonly acts?: KinActs;
  readonly fresh?: FreshTask;
  readonly assign?: (url: string) => void;
}

// KinDialog is the modal of a task's family: a new task on the task list, a child or the parent of the task of a task's
// page, of one form of fields whose form (KIN_FORMS) says what differs.
// The task is as the tasks read now have it, and so is the name typed: a task there is said, and pressed, opened or not written as the form says; any other is written, as the
// modal shows it (new). What has changed since is refused, and said, with the modal open. Saved, it closes and the
// focus goes where the form says (acts.back): the row of what was made, marked fresh, or the button of the family
// card's part; closed otherwise, back to what opened it
export function KinDialog({
  open,
  onClosed,
  pages = pagePages,
  acts = acts0,
  fresh = fresh0,
  assign = goTo,
}: KinDialogProps) {
  const hold = useHold();
  const { entries, tasks: taken } = useStore(pages.state);
  const [was, setWas] = useState<KinOpen | null>(open);
  const [name, setName] = useState('');
  const [title, setTitle] = useState('');
  const [asked, setAsked] = useState('');
  const [error, setError] = useState('');
  const nameField = useRef<HTMLInputElement>(null);
  const titleField = useRef<HTMLInputElement>(null);
  // Where the focus goes as it closes saved, set with the close
  const saved = useRef<KinBack | null>(null);
  // A task's fields as they open: none of what was typed before, nor a field's blank mark
  if (open !== was) {
    setWas(open);
    if (open) {
      setName('');
      setTitle('');
      setError('');
      setAsked(askedProjects(entries, open.task.project).chosen);
    }
  }
  const failed = Boolean(taken.error);
  const tasks = knownTasks(entries, taken);
  const form = open ? KIN_FORMS[open.form] : null;
  const project = open?.ask ? asked : (open?.task.project ?? '');
  // The task and the name's task as shown: found as it opens, as the project or name changes and as a save is refused
  // as stale, not at each read, so that a save sends the rev shown
  const ofTask = useSeenTask(open, project, open?.task.name ?? '', tasks);
  const ofName = useSeenTask(open, project, name, tasks);
  const task: KinTask = ofTask.seen ?? { ...open?.task, project };
  const typed = name.trim();
  const x = ofName.seen;
  const there = form?.there(task, typed, x) ?? null;
  // The title is asked of a new task alone: hidden, it is not required, nor marked blank. Unreadable, a task's being
  // there is not known, and nothing is asked: the save stops at what cannot be read
  const titleShown = there === null;
  // A field's blank mark goes as the modal opens, and the title's as it hides: a field hidden still marked would stop
  // every save, the browser's check failing on what cannot be seen
  useLayoutEffect(() => {
    if (open === null) return;
    nameField.current?.setCustomValidity('');
    titleField.current?.setCustomValidity('');
  }, [open]);
  useLayoutEffect(() => {
    if (!titleShown) titleField.current?.setCustomValidity('');
  }, [titleShown]);
  const suggest = useNameSuggest({
    id: 'kin-task',
    input: nameField,
    offers: (written, justTyped) =>
      form
        ? nameOffers(
            taskNames(tasks, project, (y) => form.offered(task, y), getBoot().lang),
            written,
            justTyped,
            { detail: (n) => taskOffer(tasks, project, n) },
          )
        : [],
    put: (n) => {
      // What the field was told, of a name missing, no longer holds
      nameField.current?.setCustomValidity('');
      setName(n);
      setError('');
    },
    // The modal hands the first field the focus as it opens: not over the fields under it
    onFocus: false,
  });

  // Whether the form may be sent: tasks.json read, as there is no saying whether the task is there without it, and no
  // field required that has spaces alone, which required lets through. The words typed stay, marked until typed again
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
    if (!open || !form || !ready()) return;
    const act = there?.act ?? 'write';
    if (act === 'open') {
      assign(taskHref({ project, name: typed }));
      return;
    }
    // Nothing to write, and so nothing to tell
    if (act === 'none') {
      saved.current = acts.back(open.form, task, typed);
      onClosed();
      return;
    }
    try {
      const ran = await hold.whileHeld(async () => {
        const res = await acts.write(open.form, task, typed, x, title.trim());
        if (res.status === 409) {
          ofTask.renew();
          ofName.renew();
          throw new Error(res.stale);
        }
        if (!res.ok) throw new Error(res.text);
        acts.done(open.form, task, typed, x, res.rev);
        await pages.load();
      });
      if (!ran) return;
    } catch (err) {
      setError(t('ui_save_failed', err instanceof Error ? err.message : String(err)));
      return;
    }
    // Closed and marked in one draw, after done changed the list's view: the row is drawn as the modal goes
    const back = acts.back(open.form, task, typed);
    saved.current = back;
    if ('row' in back) fresh.mark(back.row);
    onClosed();
  };

  const clearMark = (e: { currentTarget: HTMLInputElement }) => {
    e.currentTarget.setCustomValidity('');
  };
  const { names: projects } = askedProjects(entries, project);
  return (
    <Dialog
      id="kin-dialog"
      open={open !== null}
      onClosed={onClosed}
      hold={hold}
      labelledBy="kin-heading"
      afterClose={() => {
        const back = saved.current;
        saved.current = null;
        if (back === null) return false;
        return 'row' in back ? focusRow(back.row) : partButton(back.part === 'up');
      }}
    >
      <form
        id="kin"
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <DialogHead
          icon={<PlusIcon />}
          titleId="kin-heading"
          title={form?.heading(task) ?? ''}
          lead={form?.target(task, open?.ask ?? false) ?? ''}
          leadId="kin-target"
        />
        <label data-testid="field" className={FIELD} hidden={!open?.ask}>
          {t('ui_project')}
          <select
            id="kin-project"
            data-testid="text"
            className={TEXT}
            value={asked}
            data-autofocus={open?.ask ? '' : undefined}
            onChange={(e) => {
              setAsked(e.currentTarget.value);
              // What was refused was of another project
              setError('');
            }}
          >
            {projects.map((p) => (
              <option key={p} value={p}>
                {projectWord(p)}
              </option>
            ))}
          </select>
        </label>
        <label data-testid="field" className={FIELD}>
          {t('ui_task_name')}
          <input
            id="kin-task"
            data-testid="text"
            className={TEXT}
            required
            autoComplete="off"
            spellCheck={false}
            placeholder={t('ui_task_id_example')}
            data-autofocus={open?.ask ? undefined : ''}
            value={name}
            {...suggest.field}
            onInput={(e) => {
              clearMark(e);
              suggest.field.onInput(e);
              // What was refused was of another name: refused as stale, the catch says it after. On every input, as
              // onChange skips the same name typed again
              setError('');
            }}
            onChange={(e) => {
              setName(e.currentTarget.value);
            }}
          />
        </label>
        {suggest.box}
        <label data-testid="field" className={FIELD} hidden={!titleShown}>
          {t('ui_task_title')}
          <input
            ref={titleField}
            id="kin-title"
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
        <p id="kin-note" hidden={!there?.note} className="m-0 text-small text-muted">
          {there?.note ?? ''}
        </p>
        <p
          id="kin-error"
          role="alert"
          hidden={!error}
          className="m-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
        >
          {error}
        </p>
        <DialogActions>
          <DialogClose words="ui_cancel" />
          <Button id="kin-submit" kind="primary" type="submit">
            {t(there?.button ?? 'ui_child_make_button')}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
