import { useCallback, useEffect, useState, type ReactNode } from 'react';

import { Columns } from '@/components/columns';
import { backToRow } from '@/components/focus-places';
import type { Openers } from '@/components/page-menu';
import { getBoot } from '@/config/boot';
import { KinDialog, type KinOpen } from '@/features/tasks/components/kin-dialog';
import { TaskHead } from '@/features/tasks/components/task-head';
import { TaskNone } from '@/features/tasks/components/task-none';
import { TaskPageRows } from '@/features/tasks/components/task-page-rows';
import type { TaskPageView } from '@/features/tasks/hooks/use-task-page';
import { taskListHref } from '@/features/tasks/utils/task-page';
import { useBackToRow } from '@/hooks/use-back-to-row';
import { useOneColumn } from '@/hooks/use-one-column';
import { openPage as pageOpen } from '@/stores/open-page';
import { pages as pagePages } from '@/stores/pages';
import type { FocusBack } from '@/types/focus';
import type { TaskOf } from '@/types/task';

// What a task's page opens that P6c makes: the task's edit modal, and the modal that puts a page on a category. back
// gives the focus back to what opened it
export interface TaskOpeners {
  onTaskEdit: (task: TaskOf, back: FocusBack) => void;
  onAddPage: (task: TaskOf, back: FocusBack) => void;
}

export interface TaskPageScreenProps {
  // name is the task's, as the URL names it (task=); its project is the list's query's
  readonly name: string;
  // view is the task as read, which the route reads too (the bar's way up)
  readonly view: TaskPageView;
  readonly openers: Openers;
  readonly taskOpeners: TaskOpeners;
  // reader is the page beside the list: the app's, as a feature imports no other
  readonly reader: ReactNode;
}

// TaskPageScreen is a task's page (/tasks with a task named): its head and family card over its rows by category, or
// what it says with none, beside the page open, laid out by Columns; a task not there, the column alone. It writes no
// list's query (pages.list, pages.tasks): the task list does
export function TaskPageScreen({ name, view, openers, taskOpeners, reader }: TaskPageScreenProps) {
  const [kin, setKin] = useState<KinOpen | null>(null);
  useOneColumn(pageOpen);
  const back = useCallback(() => {
    backToRow({ pages: pagePages, openPage: pageOpen });
  }, []);
  useBackToRow(view.read, back);
  useEffect(() => {
    document.title = `${name} · floe`;
    return () => {
      document.title = 'floe';
    };
  }, [name]);
  const first = getBoot().states.find((s) => s.first)?.name ?? '';
  const listHref = taskListHref(view.project, view.task?.state, first);
  return (
    <>
      <Columns
        list={
          <>
            <TaskHead
              task={view.head}
              shown={view.task !== null}
              project={view.project}
              name={name}
              reads={view.reads}
              color={view.color}
              onKin={(form) => {
                if (view.task) setKin({ form, task: view.task, ask: false });
              }}
              onEdit={taskOpeners.onTaskEdit}
              onAddPage={taskOpeners.onAddPage}
            />
            <TaskPageRows task={view.task} openers={openers} />
            <TaskNone
              task={view.task}
              none={view.none}
              missing={view.missing}
              project={view.project}
              name={name}
              listHref={listHref}
            />
          </>
        }
        // A task of no page has no word beside it of a page to pick: hidden, as the reader is the app's
        reader={<div className={view.none ? 'contents [&_#read-blank]:hidden' : 'contents'}>{reader}</div>}
        alone={view.missing}
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
