import { useState } from 'react';

import { Bar } from '@/app/bar';
import { appKeys, pressDiff, pressStar, pressUp } from '@/app/keys';
import { PageReader } from '@/app/routes/reader';
import { moveRow } from '@/components/focus-places';
import type { Openers } from '@/components/page-menu';
import { FindDialog } from '@/features/find/components/find-dialog';
import { KeysDialog, type HelpTab } from '@/features/help/components/keys-dialog';
import { EditDialog, type EditTarget } from '@/features/page-edit/components/edit-dialog';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import { SettingsDialog, type SettingsTarget } from '@/features/settings/components/settings-dialog';
import { pageTags, TagDialog, type TagTarget } from '@/features/tags/components/tag-dialog';
import { AddPageDialog, type AddPageOpen } from '@/features/tasks/components/add-page-dialog';
import { MoveDialog } from '@/features/tasks/components/move-dialog';
import { TaskEditDialog, type TaskEditOpen } from '@/features/tasks/components/task-edit-dialog';
import { TaskListScreen } from '@/features/tasks/components/task-list-screen';
import { TaskPageScreen, type TaskOpeners } from '@/features/tasks/components/task-page-screen';
import { TaskSetDialog } from '@/features/tasks/components/task-set-dialog';
import { openTaskSet, type TaskSetOpen } from '@/features/tasks/stores/task-set';
import { useKeys } from '@/hooks/use-keys';
import { useOpenMenus } from '@/hooks/use-open-menu';
import { useRowKeys } from '@/hooks/use-row-keys';
import { isMac } from '@/lib/platform';
import { openPage } from '@/stores/open-page';
import { pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import { canDiff } from '@/utils/list-view';

// freeToOpen tells no modal open, which the search, the settings, the help and a category's Add a page wait for
// (ui.html openFind 6186, openThemes 6253, openKeys 6474, openAddPage 5893)
const freeToOpen = (): boolean => document.querySelector('dialog[open]') === null;

// diffGoes tells that d goes to a diff where the help opens: the bar's diff button shows over the page picked (ui.html
// 6475–6478, page-actions' own test)
function diffGoes(): boolean {
  const at = openPage.picked.get();
  const page = at ? pages.state.get().entries.find((a: Page) => a.slug === at.slug) : undefined;
  return Boolean(at && page && !at.file && canDiff(page));
}

interface Help {
  tab: HelpTab;
  diffGo: boolean;
}

const NO_READER = <ReaderPane at={null} page={null} keys={new Set()} onKey={() => undefined} />;

// TasksRoute is /tasks: the bar, the task list or a task's page (name, ?task=), and every modal they open, closed till
// then. The modals' state and the keys are its own, inside the app's DrawGuard: a draw that throws takes them with the
// screen, so that no second modal opens over one gone
export function TasksRoute({ name }: { readonly name: string | null }) {
  const [finding, setFinding] = useState(false);
  const [settings, setSettings] = useState<SettingsTarget | null>(null);
  const [help, setHelp] = useState<Help | null>(null);
  const [tags, setTags] = useState<TagTarget | null>(null);
  const [edit, setEdit] = useState<EditTarget | null>(null);
  const [set, setSet] = useState<TaskSetOpen | null>(null);
  const [taskEdit, setTaskEdit] = useState<TaskEditOpen | null>(null);
  const [adding, setAdding] = useState<AddPageOpen | null>(null);
  useOpenMenus();

  const openFind = () => {
    if (freeToOpen()) setFinding(true);
  };
  const openHelp = (tab: HelpTab) => {
    if (freeToOpen()) setHelp({ tab, diffGo: diffGoes() });
  };
  const openSettings = (target: SettingsTarget) => {
    if (freeToOpen()) setSettings(target);
  };
  const openers: Openers = {
    onTags: (a, back) => {
      setTags(pageTags(a, back));
    },
    onTheme: (a, back) => {
      openSettings({ page: a, back });
    },
    onEdit: (a, back) => {
      setEdit({ page: a, back });
    },
    onTaskSet: (a, back) => void openTaskSet(a, back, setSet),
  };
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
    findToggle: () => {
      if (finding) setFinding(false);
      else openFind();
    },
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
        onSettings={() => {
          openSettings({ page: null, back: null });
        }}
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
      <FindDialog
        open={finding}
        openers={openers}
        onClosed={() => {
          setFinding(false);
        }}
      />
      <SettingsDialog
        target={settings}
        onClosed={() => {
          setSettings(null);
        }}
      />
      <KeysDialog
        open={help?.tab ?? null}
        screen={name === null ? 'taskList' : 'taskPage'}
        diff={false}
        diffGo={help?.diffGo ?? false}
        backWords=""
        mac={isMac()}
        onClosed={() => {
          setHelp(null);
        }}
      />
      <TagDialog
        target={tags}
        onClosed={() => {
          setTags(null);
        }}
      />
      <EditDialog
        target={edit}
        onClosed={() => {
          setEdit(null);
        }}
      />
      <TaskSetDialog
        open={set}
        onClosed={() => {
          setSet(null);
        }}
      />
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
      <MoveDialog />
    </>
  );
}
