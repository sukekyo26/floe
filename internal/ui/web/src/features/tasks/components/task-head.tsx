import { Component, createRef, type ChangeEvent } from 'react';

import { Ago } from '@/components/ago';
import { BackLink } from '@/components/back-link';
import { Button } from '@/components/button';
import { focusOn, kinButton, partButton } from '@/components/focus-places';
import { ArchiveIcon, EditIcon } from '@/components/icons';
import { PickSelect } from '@/components/pick-select';
import { ProjectPill } from '@/components/project-pill';
import type { Up } from '@/components/screen-switch';
import { getBoot } from '@/config/boot';
import { kidState, TaskFamily } from '@/features/tasks/components/task-family';
import { XLink } from '@/features/tasks/components/task-row';
import { taskPageActs, type TaskPageActs } from '@/features/tasks/stores/task-page-acts';
import {
  firstShown,
  headFocus,
  stateShown,
  type HeadFocused,
  type StateShown,
} from '@/features/tasks/utils/head-focus';
import { stateWord } from '@/features/tasks/utils/kin-forms';
import { familyOf } from '@/features/tasks/utils/task-page';
import { projectWord, t } from '@/lib/t';
import type { FocusBack } from '@/types/focus';
import type { TaskOf } from '@/types/task';

export interface TaskHeadProps {
  // task is the task the head draws: as last drawn once gone, so that a button pressed then acts on it; null before any
  // is read
  readonly task: TaskOf | null;
  readonly shown: boolean;
  // up is the page's way up to the task list
  readonly up: Up;
  // project and name are the page's, as its URL names them
  readonly project: string;
  readonly name: string;
  // reads is the reads of the list ended (pages.ended): the choices are drawn anew at each
  readonly reads: number;
  // color is the project's
  readonly color: string;
  readonly onKin: (form: 'child' | 'parent') => void;
  readonly onEdit: (task: TaskOf, back: FocusBack) => void;
  readonly acts?: TaskPageActs;
}

interface Snapshot {
  was: HeadFocused | null;
  el: Element | null;
}

// A browser without base-select has no :open, and its own list goes on over what is drawn
const isOpen = (sel: HTMLSelectElement | null): boolean =>
  sel !== null && CSS.supports('selector(:open)') && sel.matches(':open');

// TaskHead is over the rows of a task's page: the way back to the task list, its name and title and its Edit in one row,
// its state and how to change it, its time, the button that archives it, its project, then its links and its family
// card. Drawn anew, the focus it had goes where headFocus says, taken just before React changes it
// LIMIT: a browser without :open redraws the choices while its list is open: what is chosen goes with the rev drawn last,
// refused or written as of then. Hold them some other way if such a browser comes to matter
export class TaskHead extends Component<TaskHeadProps, { shown: StateShown | null }> {
  override state: { shown: StateShown | null } = { shown: null };
  private readonly select = createRef<HTMLSelectElement>();

  override componentDidMount() {
    this.follow();
  }

  override getSnapshotBeforeUpdate(): Snapshot {
    const el = document.activeElement;
    if (!el) return { was: null, el: null };
    if (el === this.select.current) return { was: { on: 'state' }, el };
    if (el.id === 'task-parent-add' || el.id === 'task-parent-change') return { was: { on: 'parent' }, el };
    const task = el.closest('#task-family') ? el.getAttribute('data-task') : null;
    if (task === null) return { was: null, el };
    return { was: { on: 'kin', task, unlink: el.getAttribute('data-testid') === 'unlink' }, el };
  }

  override componentDidUpdate(_prev: TaskHeadProps, _state: unknown, snap: Snapshot) {
    const to = headFocus(snap.was, {
      stateHidden: Boolean(this.select.current?.hidden),
      parentHidden: Boolean(snap.el?.closest('[hidden]')),
      rebuilt: snap.el !== null && !snap.el.isConnected,
    });
    if (to?.to === 'archive') focusOn(document.getElementById('task-archive'));
    else if (to?.to === 'part-up') partButton(true);
    else if (to?.to === 'kin') kinButton(to.task, to.unlink);
    this.follow();
  }

  // follow takes the task's state as stateShown says, at each draw
  private follow() {
    const { task, reads } = this.props;
    if (!task) return;
    const now = this.state.shown ?? firstShown(task, reads);
    const next = stateShown(now, task, reads, isOpen(this.select.current));
    if (next !== this.state.shown) this.setState({ shown: next });
  }

  // A change is sent at once, of the rev drawn with the value it was chosen over; the value chosen shows while it goes
  private readonly change = async (e: ChangeEvent<HTMLSelectElement>) => {
    const { project, name, acts = taskPageActs } = this.props;
    const now = this.state.shown;
    if (!now) return;
    const value = e.currentTarget.value;
    // A select keeps the keys: h, j and k go on from the page
    e.currentTarget.blur();
    this.setState({ shown: { ...now, chosen: value } });
    // The answer is drawn before the list is read again: written, of its rev; refused, the value drawn back
    await acts.changeState(project, name, now.rev, value, (rev) => {
      this.setState(({ shown }) => ({
        shown: shown && (rev === null ? { ...shown, chosen: null } : { ...shown, state: value, rev, chosen: null }),
      }));
    });
  };

  override render() {
    const { task, up, shown: visible, project, name, color, onKin, onEdit, acts = taskPageActs } = this.props;
    const { lang, states } = getBoot();
    const shown = this.state.shown;
    const drawn = shown?.state ?? task?.state ?? '';
    const back = task?.state === 'archived';
    const family = task
      ? familyOf(
          task,
          states.map((s) => s.name),
          lang,
        )
      : null;
    return (
      <div
        id="task-head"
        hidden={!visible}
        className="grid flex-none gap-[0.35rem] border-b border-line bg-panel px-4 pt-2 pb-[0.65rem]"
      >
        {/* The mark at the title's left edge, in line with the rows under it; the mark and Edit in the middle of the title,
            however many lines it takes. The ID is a line of its own, so that a title that wraps starts each line where
            its first starts */}
        <div className="-ms-3 flex items-center gap-2">
          <BackLink id="task-up" up={up} label={t('ui_tasks_back')} />
          <h2 id="task-title" className="m-0 flex-1 text-[1rem] leading-[1.4] font-bold wrap-anywhere">
            <span data-testid="tname" className="block font-mono text-body font-semibold text-muted">
              {name}
            </span>
            {task?.title ?? ''}
          </h2>
          <Button
            id="task-edit"
            label={t('ui_task_edit_named', name)}
            onClick={() => {
              if (task) onEdit(task, () => focusOn(document.getElementById('task-edit')));
            }}
          >
            <EditIcon />
            {t('ui_edit')}
          </Button>
        </div>
        <div data-testid="tstate" className="flex flex-wrap items-center gap-x-[0.6rem] gap-y-1 text-small text-muted">
          <PickSelect
            ref={this.select}
            id="task-state"
            compact
            label={t('ui_task_state')}
            value={shown?.chosen ?? drawn}
            state={drawn}
            hidden={drawn === 'archived'}
            onChange={(e) => void this.change(e)}
          >
            {states
              .filter((s) => s.name !== 'archived')
              .map((s) => (
                <option key={s.name} value={s.name}>
                  {stateWord(s.name)}
                </option>
              ))}
          </PickSelect>
          <span
            id="task-archived"
            data-testid="kid-state"
            hidden={drawn !== 'archived'}
            className={kidState('archived')}
          >
            {t('ui_state_archived')}
          </span>
          <time id="task-at" hidden={!task?.at} dateTime={task?.at} className="font-mono text-label">
            {task?.at ? <Ago iso={task.at} /> : null}
          </time>
          <Button
            id="task-archive"
            hidden={!back && task?.state !== 'done'}
            label={t(back ? 'ui_unarchive_named' : 'ui_archive_named', name)}
            onClick={(e) => {
              if (task) void acts.archive(e.detail, task, !back);
            }}
          >
            <ArchiveIcon />
            <span data-testid="label">{t(back ? 'ui_unarchive' : 'ui_archive')}</span>
          </Button>
          <ProjectPill
            id="task-project"
            word={projectWord(project)}
            color={color}
            place="ms-auto max-w-full text-label"
          />
        </div>
        <div
          id="task-link"
          hidden={!task?.urls?.length}
          className="mt-[0.35rem] grid min-w-0 justify-items-start gap-[0.2rem]"
        >
          {(task?.urls ?? []).map((url) => (
            <XLink key={url} url={url} words={url} />
          ))}
        </div>
        <TaskFamily
          task={task}
          family={family}
          onKin={onKin}
          onUnlink={(child, parent, fromKeys) => void acts.unlink(child, parent, name, fromKeys)}
        />
      </div>
    );
  }
}
