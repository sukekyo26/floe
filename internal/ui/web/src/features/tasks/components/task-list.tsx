import { useCallback } from 'react';

import { showColumn } from '@/components/columns';
import { backToRow } from '@/components/focus-places';
import { ListHeadBox } from '@/components/list-head-box';
import { Rows, type RowItem } from '@/components/rows';
import { ShelfWhat } from '@/components/shelf-what';
import { Pins } from '@/features/tasks/components/pins';
import { TaskListEmpty } from '@/features/tasks/components/task-list-empty';
import { TaskRow } from '@/features/tasks/components/task-row';
import type { TaskListView } from '@/features/tasks/hooks/use-task-list';
import { freshTask as fresh0, type FreshTask } from '@/features/tasks/stores/fresh-task';
import { stateWord } from '@/features/tasks/utils/kin-forms';
import { taskProjects, taskRanks, taskRowKey } from '@/features/tasks/utils/task-list';
import { useBackToRow } from '@/hooks/use-back-to-row';
import { useStore } from '@/hooks/use-store';
import { projectWord, t } from '@/lib/t';
import { columns as pageColumns, type ColumnsStore } from '@/stores/columns';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { projectColor } from '@/utils/project-color';

export interface TaskListProps {
  readonly view: TaskListView;
  // onAdd opens the modal that adds a task
  readonly onAdd: () => void;
  readonly pages?: Pages;
  readonly columns?: ColumnsStore;
  readonly fresh?: FreshTask;
}

// TaskList is the task list's column: the pins of its project, its head, the project's tasks of the view, latest change
// first, or the words why there are none. Nothing of the shelf's but the view and the project narrows it. As it is
// first drawn, the focus goes back to the row of the task, or the pin, it was come back to from
export function TaskList({ view, onAdd, pages = pagePages, columns = pageColumns, fresh = fresh0 }: TaskListProps) {
  const { read, failed, tasks, entries, project, shown, pinned, state } = view;
  const freshKey = useStore(fresh.key);
  const { open: shelfOpen } = useStore(columns.columns);
  const back = useCallback(() => {
    backToRow({ pages });
  }, [pages]);
  useBackToRow(read, back);
  const tint = project ? projectColor(project, taskRanks(entries, taskProjects(tasks, entries))) : null;
  const words = project ? projectWord(project) : '';
  const rows: RowItem[] = shown.map((task, i) => {
    const key = taskRowKey(task.project, task.name);
    return {
      kind: 'row',
      key,
      titled: true,
      render: (kept) => <TaskRow task={task} index={i} kept={kept} fresh={key === freshKey} />,
    };
  });
  return (
    <>
      {/* No project yet (the tasks unreadable, none kept) has no pins of its own to say */}
      <Pins pinned={pinned} hidden={!project || state.nothing} pages={pages} />
      <ListHeadBox hidden={state.nothing}>
        <ShelfWhat
          words={words}
          tint={tint}
          hidden={shelfOpen || words === ''}
          onShow={() => {
            showColumn('shelf', true, columns);
          }}
        />
        <span id="count" className="me-auto min-w-0 truncate">
          {t('ui_tasks_head_newest', shown.length)}
        </span>
      </ListHeadBox>
      <Rows items={rows} selecting={false} pages={pages} />
      <TaskListEmpty
        nothing={state.nothing}
        failed={read && failed}
        none={
          state.none
            ? state.gone
              ? t('ui_task_none', projectWord(project))
              : t('ui_task_none_state', stateWord(view.view))
            : null
        }
        other={state.gone}
        onAdd={onAdd}
      />
    </>
  );
}
