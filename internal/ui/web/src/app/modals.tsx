import { useState, type ReactNode } from 'react';

import type { Openers } from '@/components/page-menu';
import { FindDialog } from '@/features/find/components/find-dialog';
import { KeysDialog, type HelpTab } from '@/features/help/components/keys-dialog';
import { EditDialog, type EditTarget } from '@/features/page-edit/components/edit-dialog';
import { SettingsDialog, type SettingsTarget } from '@/features/settings/components/settings-dialog';
import { pageTags, TagDialog, type TagTarget } from '@/features/tags/components/tag-dialog';
import { MoveDialog } from '@/features/tasks/components/move-dialog';
import { TaskSetDialog } from '@/features/tasks/components/task-set-dialog';
import { openTaskSet, type TaskSetOpen } from '@/features/tasks/stores/task-set';
import { useOpenMenus } from '@/hooks/use-open-menu';
import { isMac } from '@/lib/platform';
import { openPage, type OpenPage } from '@/stores/open-page';
import { pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { Screen } from '@/types/screen';
import { canDiff } from '@/utils/list-view';

// freeOf tells no modal open in opened, the ids of those open, but closing: the one closed in the same press, which is
// still open till React draws (the search, as its foot's menu opens the settings)
export const freeOf = (opened: readonly string[], closing = ''): boolean => opened.every((id) => id === closing);

// freeToOpen tells no modal open, which the search, the settings, the help and a category's Add a page wait for
export const freeToOpen = (closing = ''): boolean =>
  freeOf(
    [...document.querySelectorAll('dialog[open]')].map((d) => d.id),
    closing,
  );

// diffGoes tells that d goes to a diff where the help opens: the bar's diff button shows over the page open, whole or
// beside the list (page-actions' own test). Not in a diff, nor for a page whole not found
export function diffGoes(
  open: Pick<OpenPage, 'frameAt' | 'picked' | 'diff'> = openPage,
  entries: readonly Page[] = pages.state.get().entries,
): boolean {
  const at = open.frameAt() ?? open.picked.get();
  const page = at ? entries.find((a) => a.slug === at.slug) : undefined;
  return Boolean(at && page && !page.missing && !at.file && !open.diff() && canDiff(page));
}

// FrameHelp is what a page whole's help says of it: whether it shows a diff (its n and p), and where h goes back to
export interface FrameHelp {
  readonly diff: boolean;
  readonly backWords: string;
}

interface Help {
  tab: HelpTab;
  diffGo: boolean;
}

export interface ScreenModals {
  readonly openers: Openers;
  // fromFind is openers for the search's foot menu, which opens them as the search closes
  readonly fromFind: Openers;
  readonly openFind: () => void;
  readonly findToggle: () => void;
  readonly openHelp: (tab: HelpTab) => void;
  readonly openSettings: () => void;
  readonly openTags: (target: TagTarget) => void;
  // modals are the modals every screen holds, closed till opened
  readonly modals: ReactNode;
}

// useScreenModals is the modals every screen opens and their ways in: the search, the settings, the help, the tags, the
// edit, a task to set and the question of a move, with the one menu open (useOpenMenus). A screen calls it once, inside
// the app's DrawGuard: a draw that throws takes the modals' state with the screen, so that no second opens over one gone
export function useScreenModals(screen: Screen, frame?: FrameHelp): ScreenModals {
  const [finding, setFinding] = useState(false);
  const [settings, setSettings] = useState<SettingsTarget | null>(null);
  const [help, setHelp] = useState<Help | null>(null);
  const [tags, setTags] = useState<TagTarget | null>(null);
  const [edit, setEdit] = useState<EditTarget | null>(null);
  const [set, setSet] = useState<TaskSetOpen | null>(null);
  useOpenMenus();

  const openFind = () => {
    if (freeToOpen()) setFinding(true);
  };
  const openHelp = (tab: HelpTab) => {
    if (freeToOpen()) setHelp({ tab, diffGo: diffGoes() });
  };
  const openSettingsOn = (target: SettingsTarget, closing = '') => {
    if (freeToOpen(closing)) setSettings(target);
  };
  const openers: Openers = {
    onTags: (a, back) => {
      setTags(pageTags(a, back));
    },
    onTheme: (a, back) => {
      openSettingsOn({ page: a, back });
    },
    onEdit: (a, back) => {
      setEdit({ page: a, back });
    },
    onTaskSet: (a, back) => void openTaskSet(a, back, setSet),
  };
  // The search's foot menu opens them as the search closes: the settings past the search still open
  const fromFind: Openers = {
    ...openers,
    onTheme: (a, back) => {
      openSettingsOn({ page: a, back }, 'find');
    },
  };

  return {
    openers,
    fromFind,
    openFind,
    findToggle: () => {
      if (finding) setFinding(false);
      else openFind();
    },
    openHelp,
    openSettings: () => {
      openSettingsOn({ page: null, back: null });
    },
    openTags: setTags,
    modals: (
      <>
        <FindDialog
          open={finding}
          openers={fromFind}
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
          screen={screen}
          diff={frame?.diff ?? false}
          diffGo={help?.diffGo ?? false}
          backWords={frame?.backWords ?? ''}
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
        <MoveDialog />
      </>
    ),
  };
}
