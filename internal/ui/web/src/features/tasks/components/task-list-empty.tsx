import { Button } from '@/components/button';
import { EmptyState } from '@/components/empty-state';
import { PlusIcon } from '@/components/icons';
import { t } from '@/lib/t';

const LINK = 'text-fg';

export interface TaskListEmptyProps {
  // nothing: no task nor pin anywhere; failed: the tasks unreadable; none: no task to show, its words those of none,
  // other telling the project gone
  readonly nothing: boolean;
  readonly failed: boolean;
  readonly none: string | null;
  readonly other: boolean;
  readonly onAdd: () => void;
}

// TaskListEmpty is what the task list says in place of its rows: all drawn, the one shown hidden no more, as the tests
// find each by its id and watch them
export function TaskListEmpty({ nothing, failed, none, other, onAdd }: TaskListEmptyProps) {
  return (
    <>
      <EmptyState id="tasks-none" hidden={none === null}>
        <strong>{none ?? ''}</strong>
        <p>
          <a href="/tasks" id="tasks-other" hidden={!other} className={LINK}>
            {t('ui_tasks_other')}
          </a>
        </p>
      </EmptyState>
      <EmptyState id="tasks-failed" hidden={!failed}>
        <strong>{t('ui_tasks_unreadable')}</strong>
        <p>
          <a href="/" className={LINK}>
            {t('ui_to_page_list')}
          </a>
        </p>
      </EmptyState>
      <EmptyState id="tasks-empty" hidden={!nothing}>
        <strong>{t('ui_tasks_empty')}</strong>
        <p>{t('ui_tasks_empty_hint')}</p>
        <p>
          <a href="/" className={LINK}>
            {t('ui_to_page_list')}
          </a>
        </p>
        <p>
          <Button id="task-add-empty" adds onClick={onAdd}>
            <PlusIcon />
            {t('ui_task_add')}
          </Button>
        </p>
      </EmptyState>
    </>
  );
}
