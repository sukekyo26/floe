import { useState } from 'react';

import { Bar } from '@/app/bar';
import { appKeys, pressDiff, pressStar, pressUp } from '@/app/keys';
import { freeToOpen, useScreenModals } from '@/app/modals';
import { PageReader } from '@/app/routes/reader';
import { moveRow } from '@/components/focus-places';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import { AddPageDialog, type AddPageOpen } from '@/features/tasks/components/add-page-dialog';
import { TaskEditDialog, type TaskEditOpen } from '@/features/tasks/components/task-edit-dialog';
import { TaskListScreen } from '@/features/tasks/components/task-list-screen';
import { TaskPageScreen, type TaskOpeners } from '@/features/tasks/components/task-page-screen';
import { useKeys } from '@/hooks/use-keys';
import { useRowKeys } from '@/hooks/use-row-keys';

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
    onAddPage: (task, category, back) => {
      if (freeToOpen()) setAdding({ task, category, back });
    },
  };

  const rows = useRowKeys({ move: moveRow });
  const keys = appKeys({
    rows,
    taskPage: name !== null,
    find: openFind,
    findToggle,
    help: () => {
      openHelp('keys');
    },
    star: pressStar,
    diff: pressDiff,
    up: pressUp,
  });
  useKeys(keys);

  return (
    <>
      <Bar
        tasks
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
