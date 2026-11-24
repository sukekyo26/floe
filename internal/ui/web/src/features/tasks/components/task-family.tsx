import { Button } from '@/components/button';
import { IconButton } from '@/components/icon-button';
import { EditIcon, KidsIcon, ParentIcon, PlusIcon, UnlinkIcon } from '@/components/icons';
import { PartHead } from '@/features/tasks/components/part-head';
import { ReadMark } from '@/components/page-row';
import { stateWord } from '@/features/tasks/utils/kin-forms';
import { unreadOf } from '@/features/tasks/utils/task-list';
import { familySaid, type Family } from '@/features/tasks/utils/task-page';
import { t } from '@/lib/t';
import type { TaskOf } from '@/types/task';
import { taskHref } from '@/utils/page-urls';

// KID_STATE is a task's state as a word on its ground: a row of the card, an archived task's head
export const kidState = (state: string): string =>
  `flex-none rounded-small px-[0.4rem] text-label font-bold whitespace-nowrap ${
    state === 'doing' ? 'bg-chg-soft text-chg' : state === 'done' ? 'bg-ins-soft text-ins' : 'bg-subtle text-muted'
  }`;

// The rows share the columns of their part, so that the states are as wide as the widest and the names stand in line:
// state, name (cut past 20%), title, unread dot, the button that takes it off
const ROWS = 'grid grid-cols-[max-content_fit-content(20%)_minmax(0,1fr)_max-content_max-content] gap-x-2';
const PART = 'grid gap-[0.1rem]';
const HEAD = 'pb-1 ps-2';
// As a row of the list, reached by keyboard the row is outlined as a whole, inside it; only what the keys reach is
// grounded. With a mouse, the button that takes it off shows while the row is pointed at or reached (opacity alone, so
// it keeps its place)
const LINE =
  'col-span-full grid min-h-9 grid-cols-subgrid items-center rounded-[calc(var(--radius-menu)-var(--spacing-menu-pad))] ps-2 pe-1 has-focus-visible:bg-hover has-[[data-testid=kin-row]:focus-visible]:outline-2 has-[[data-testid=kin-row]:focus-visible]:-outline-offset-2 has-[[data-testid=kin-row]:focus-visible]:outline-faint can-hover:hover:bg-hover can-hover:[&_[data-testid=unlink]]:transition-opacity can-hover:[&_[data-testid=unlink]]:duration-120 can-hover:[&:not(:hover,:focus-within)_[data-testid=unlink]]:opacity-0';

interface KinLineProps {
  // x is the row's task; child the one taken off parent by its button
  readonly x: TaskOf;
  readonly child: TaskOf;
  readonly parent: string;
  readonly onUnlink: (child: TaskOf, parent: string, fromKeys: boolean) => void;
}

// KinLine is a row of the card: a link to the task's page, and the button that takes the child off its parent, outside
// the link
function KinLine({ x, child, parent, onUnlink }: KinLineProps) {
  const unread = unreadOf(x) > 0;
  const named = [x.name, x.title].filter(Boolean);
  const label = child === x ? t('ui_task_unlink_kid', x.name) : t('ui_task_unlink_up', parent);
  return (
    <div data-testid="kin-line" className={LINE}>
      <a
        data-testid="kin-row"
        href={taskHref(x)}
        data-task={x.name}
        data-unread={unread ? 'true' : undefined}
        // Whole, as the row cuts a long name and title; read name first, as the row is drawn state first
        title={named.join('\n')}
        aria-label={t('ui_task_kin_row', named.join(' '), stateWord(x.state))}
        className="group/kin col-[1/-2] grid grid-cols-subgrid items-center self-stretch text-small text-fg no-underline focus-visible:outline-none data-unread:font-bold"
      >
        <span data-testid="kid-state" className={`text-center ${kidState(x.state)}`}>
          {stateWord(x.state)}
        </span>
        <span
          data-testid="tname"
          className="m-0 min-w-0 truncate font-mono text-small font-semibold text-muted can-hover:group-hover/kin:underline"
        >
          {x.name}
        </span>
        <span data-testid="kin-title" className="min-w-0 truncate can-hover:group-hover/kin:underline">
          {x.title ?? ''}
        </span>
        <ReadMark unread={unread} />
      </a>
      <IconButton
        size="kin"
        testid="unlink"
        task={x.name}
        label={label}
        // From the keys (no pointer, so no detail), the focus goes to the toast's way back
        onClick={(e) => {
          onUnlink(child, parent, e.detail === 0);
        }}
      >
        <UnlinkIcon />
      </IconButton>
    </div>
  );
}

export interface TaskFamilyProps {
  readonly task: TaskOf | null;
  readonly family: Family | null;
  readonly onKin: (form: 'child' | 'parent') => void;
  readonly onUnlink: (child: TaskOf, parent: string, fromKeys: boolean) => void;
}

// TaskFamily is a task's family card over its pages, off #list: its parent, offered while it has no children, and its
// children, offered while it has no parent. Every part is there, hidden as it does not show, and its rows are made anew
// only as what the card says changes (familySaid), the focus kept by the head
export function TaskFamily({ task, family, onKin, onUnlink }: TaskFamilyProps) {
  const said = task && family ? familySaid(task, family) : '';
  const kids = family?.kids ?? [];
  return (
    <div
      id="task-family"
      hidden={!family?.shown}
      className="mt-[0.15rem] grid rounded-menu border border-line bg-frame-bg p-menu-pad shadow-sm"
    >
      <div id="task-up-part" hidden={!family?.ups} className={PART}>
        <PartHead
          level={3}
          place={HEAD}
          words={
            <>
              <ParentIcon />
              {t('ui_task_parent')}
            </>
          }
        >
          <Button
            id="task-parent-add"
            adds
            label={t('ui_up_add_named')}
            hidden={Boolean(task?.parent)}
            onClick={() => {
              onKin('parent');
            }}
          >
            <PlusIcon />
            {t('ui_task_child_add')}
          </Button>
          <Button
            id="task-parent-change"
            adds
            label={t('ui_up_change_named')}
            hidden={!task?.parent}
            onClick={() => {
              onKin('parent');
            }}
          >
            <EditIcon />
            {t('ui_task_up_change')}
          </Button>
        </PartHead>
        <div id="task-up-rows" className={ROWS}>
          {task && family?.up ? (
            <KinLine
              key={`${said}:${family.up.name}`}
              x={family.up}
              child={task}
              parent={family.up.name}
              onUnlink={onUnlink}
            />
          ) : null}
        </div>
      </div>
      <div
        id="task-kids-part"
        hidden={!family?.kidsShown}
        className={`${PART} [#task-up-part:not([hidden])+&]:mt-menu-pad [#task-up-part:not([hidden])+&]:border-t [#task-up-part:not([hidden])+&]:border-line [#task-up-part:not([hidden])+&]:pt-menu-pad`}
      >
        <PartHead
          level={3}
          place={HEAD}
          words={
            <>
              <KidsIcon />
              <span id="task-family-word">
                {kids.length > 0 ? t('ui_task_kids_head', kids.length) : t('ui_task_kids_word')}
              </span>
            </>
          }
        >
          <Button
            id="task-child"
            adds
            label={t('ui_child_add_named')}
            hidden={!family?.adds}
            onClick={() => {
              onKin('child');
            }}
          >
            <PlusIcon />
            {t('ui_task_child_add')}
          </Button>
        </PartHead>
        <div id="task-family-rows" className={ROWS}>
          {task
            ? kids.map((x) => (
                <KinLine key={`${said}:${x.name}`} x={x} child={x} parent={task.name} onUnlink={onUnlink} />
              ))
            : null}
        </div>
      </div>
    </div>
  );
}
