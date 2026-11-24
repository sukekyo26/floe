import { useEffect, useState, type ReactNode } from 'react';

import { Columns, ShelfFoot } from '@/components/columns';
import { KinDialog, type KinOpen } from '@/features/tasks/components/kin-dialog';
import { TaskList } from '@/features/tasks/components/task-list';
import { TaskShelf } from '@/features/tasks/components/task-shelf';
import { useTaskList } from '@/features/tasks/hooks/use-task-list';
import { freshTask } from '@/features/tasks/stores/fresh-task';
import { holdProject } from '@/features/tasks/stores/hold-project';

export interface TaskListScreenProps {
  // reader is the page beside the list, of none here but there all the same, hidden: the app's, as a feature imports no
  // other
  readonly reader: ReactNode;
}

// TaskListScreen is the task list (/tasks with no task named): its shelf, its column and the modal that adds a task,
// laid out by Columns; with no task nor pin anywhere, the column alone, which says how to make one
export function TaskListScreen({ reader }: TaskListScreenProps) {
  const view = useTaskList();
  const [kin, setKin] = useState<KinOpen | null>(null);
  useEffect(() => holdProject(), []);
  useEffect(() => freshTask.follow(), []);
  const onAdd = () => {
    setKin({ form: 'add', task: { project: view.project }, ask: view.state.nothing });
  };
  return (
    <>
      <Columns
        shelf={
          <TaskShelf
            tasks={view.tasks}
            entries={view.entries}
            project={view.project}
            view={view.view}
            failed={view.failed}
            addHidden={view.state.addHidden}
            onAdd={onAdd}
            foot={<ShelfFoot />}
          />
        }
        list={<TaskList view={view} onAdd={onAdd} />}
        reader={reader}
        alone={view.state.nothing}
      />
      <KinDialog
        open={kin}
        onClosed={() => {
          setKin(null);
        }}
      />
    </>
  );
}
