import { useState } from 'react';

import { Bar } from '@/app/bar';
import { appKeys, pressDiff, pressStar, pressUp } from '@/app/keys';
import { freeToOpen, useScreenModals } from '@/app/modals';
import { PageReader } from '@/app/routes/reader';
import { moveRow } from '@/components/focus-places';
import type { Up } from '@/components/screen-switch';
import { getBoot } from '@/config/boot';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import { AddPageDialog, type AddPageOpen } from '@/features/tasks/components/add-page-dialog';
import { TaskEditDialog, type TaskEditOpen } from '@/features/tasks/components/task-edit-dialog';
import { TaskListScreen } from '@/features/tasks/components/task-list-screen';
import { TaskPageScreen, type TaskOpeners } from '@/features/tasks/components/task-page-screen';
import { useTaskPage } from '@/features/tasks/hooks/use-task-page';
import { taskRowKey } from '@/features/tasks/utils/task-list';
import { taskListHref } from '@/features/tasks/utils/task-page';
import { useKeys } from '@/hooks/use-keys';
import { useRowKeys } from '@/hooks/use-row-keys';
import { listQuery } from '@/stores/list-query';

const NO_READER = <ReaderPane at={null} page={null} keys={new Set()} onKey={() => undefined} />;

// TasksRoute is /tasks: the bar, the task list or a task's page (name, ?task=), the modals every screen opens
// (useScreenModals) and the task page's own, closed till then. They and the keys are its own, inside the app's DrawGuard
export function TasksRoute({ name }: { readonly name: string | null }) {
  const [taskEdit, setTaskEdit] = useState<TaskEditOpen | null>(null);
  const [adding, setAdding] = useState<AddPageOpen | null>(null);
  const screen = name === null ? 'taskList' : 'taskPage';
  const { openers, openFind, findToggle, openHelp, openSettings, modals } = useScreenModals(screen);

  const taskOpeners: TaskOpeners = {
    onTaskEdit: (task, back) => {
      setTaskEdit({ task, back });
    },
    onAddPage: (task, back) => {
      if (freeToOpen()) setAdding({ task, back });
    },
  };
  // A task's page: its task as read, and its way up, the bar's task list and the head's back link: to the list at the
  // task's state, its row kept to come back to (it stands as the page opens: h may be pressed before the first answer)
  const view = useTaskPage(name ?? '');
  const first = getBoot().states.find((s) => s.first)?.name ?? '';
  const up: Up = {
    href: taskListHref(view.project, view.head?.state, first),
    onPress: () => {
      listQuery.keepFrom(taskRowKey(view.project, name ?? ''));
    },
  };

  const rows = useRowKeys({ move: moveRow });
  const keys = appKeys({
    rows,
    find: openFind,
    findToggle,
    help: () => {
      openHelp('keys');
    },
    star: pressStar,
    diff: pressDiff,
    up:
      name === null
        ? undefined
        : () => {
            pressUp('to-tasks');
          },
  });
  useKeys(keys);

  return (
    <>
      <Bar
        screen={name === null ? 'taskList' : 'taskPage'}
        taskUp={name === null ? undefined : up}
        onFind={openFind}
        onHelp={() => {
          openHelp('about');
        }}
        onSettings={openSettings}
      />
      {/* After the mistake's words, which stand outside the DrawGuard and so later in the document (ErrorBar) */}
      <div className="order-1 flex min-h-0 flex-1 flex-col">
        {name === null ? (
          <TaskListScreen reader={NO_READER} />
        ) : (
          <TaskPageScreen
            name={name}
            view={view}
            up={up}
            openers={openers}
            taskOpeners={taskOpeners}
            reader={
              <PageReader
                openers={openers}
                keys={new Set(Object.keys(keys))}
                onKey={(key) => {
                  keys[key]?.();
                }}
              />
            }
          />
        )}
      </div>
      {modals}
      <TaskEditDialog
        open={taskEdit}
        onClosed={() => {
          setTaskEdit(null);
        }}
      />
      <AddPageDialog
        open={adding}
        onClosed={() => {
          setAdding(null);
        }}
      />
    </>
  );
}
