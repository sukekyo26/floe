import { useState } from 'react';

import { Button } from '@/components/button';
import { EmptyState } from '@/components/empty-state';
import { DeleteIcon } from '@/components/icons';
import { TextLink } from '@/components/text-link';
import { taskPageActs, type TaskPageActs } from '@/features/tasks/stores/task-page-acts';
import { useArm } from '@/hooks/use-arm';
import { goTo } from '@/lib/platform';
import { t } from '@/lib/t';
import type { TaskOf } from '@/types/task';

export interface TaskNoneProps {
  readonly task: TaskOf | null;
  // none: the task has no page; missing: read, and not there
  readonly none: boolean;
  readonly missing: boolean;
  readonly project: string;
  readonly name: string;
  // listHref is the task list it goes back to (taskListHref)
  readonly listHref: string;
  readonly acts?: TaskPageActs;
  readonly assign?: (url: string) => void;
}

// TaskNone is what a task's page says in place of its rows: a task of no page, with the way to remove its row by a
// press again, as a delete asks, unless it has children (ForgetTask refuses it); or a task not there, with the way back
// to the list. Both drawn hidden till shown, as the tests find them by id
export function TaskNone({
  task,
  none,
  missing,
  project,
  name,
  listHref,
  acts = taskPageActs,
  assign = goTo,
}: TaskNoneProps) {
  const arm = useArm(`${project}\n${name}`);
  const [error, setError] = useState('');
  const kids = none && (task?.children.length ?? 0) > 0;
  const forget = async () => {
    if (arm.press()) return;
    setError('');
    const why = await acts.forget(project, name);
    if (!why) {
      assign(listHref);
      return;
    }
    setError(t('ui_task_forget_failed', why));
    arm.disarm();
  };
  return (
    <>
      <EmptyState id="task-none" hidden={!none}>
        <strong>{t('ui_task_no_pages')}</strong>
        <p id="task-delete-box" hidden={kids}>
          <Button id="task-delete" tone="danger" armed={arm.armed} onClick={() => void forget()} onBlur={arm.disarm}>
            <DeleteIcon />
            <span data-testid="label">{t(arm.armed ? 'ui_task_forget_sure' : 'ui_task_forget')}</span>
          </Button>
        </p>
        <p
          id="task-delete-error"
          role="alert"
          hidden={kids || !error}
          className="rounded-menu bg-danger-soft px-[0.9rem] py-[0.65rem] whitespace-pre-wrap text-danger"
        >
          {error}
        </p>
        <p id="task-none-kids" hidden={!kids}>
          {t('ui_task_none_kids')}
        </p>
      </EmptyState>
      <EmptyState id="task-missing" hidden={!missing}>
        <strong>{t('ui_task_missing', name)}</strong>
        <p>
          <TextLink id="task-missing-up" href={listHref}>
            {t('ui_tasks_back')}
          </TextLink>
        </p>
      </EmptyState>
    </>
  );
}
