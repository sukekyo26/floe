import { useEffect, useState } from 'react';

import {
  menuBack,
  prepareArchive,
  prepareTaskOut,
  restoreFocus,
  rowMenuId,
  unreadFocus,
} from '@/components/focus-places';
import {
  ArchiveIcon,
  CopyIcon,
  DeleteIcon,
  DoneIcon,
  DownloadIcon,
  EditIcon,
  OpenIcon,
  PinIcon,
  SelectIcon,
  StarIcon,
  TagIcon,
  TaskOutIcon,
  TaskSetIcon,
  ThemeIcon,
  ToIcon,
  UnreadIcon,
} from '@/components/icons';
import { MenuItem, MoreMenu } from '@/components/more-menu';
import { useArm } from '@/hooks/use-arm';
import { t } from '@/lib/t';
import { closedThen, menus } from '@/stores/menus';
import { pageActions as pageActionStore, type PageActions } from '@/stores/page-actions';
import type { FocusBack } from '@/types/focus';
import type { Page } from '@/types/page';
import { hasPage, removal } from '@/utils/list-view';
import { downloadHref, rawHref, taskHref, type PageAt } from '@/utils/page-urls';

// Where a page's actions stand: a row, or the bar over a page beside the list, in the frame, or in the search's foot
export type Place = 'row' | 'list' | 'frame' | 'find';

// What opens a modal of another feature on the page, given back the way to the menu's button as it closes
export interface Openers {
  onTags: (a: Page, back: FocusBack) => void;
  onTheme: (a: Page, back: FocusBack) => void;
  // The edit is given the way back by the page's slug as it is then: renamed, its row's menu is under the new one
  onEdit: (a: Page, back: (slug: string) => boolean) => void;
  onTaskSet: (a: Page, back: FocusBack) => void;
}

export interface PageMenuProps {
  readonly page: Page;
  // at is what of the page it acts on: its version, for the links
  readonly at: PageAt;
  readonly place: Place;
  readonly menuId: string;
  readonly openers: Openers;
  // onPick ticks the row's box (a row's menu alone)
  readonly onPick?: () => void;
  // onRemoved tells the page deleted (the list's selection lets it go)
  readonly onRemoved?: () => void;
  // taskShown is the task whose page this is, whose own way to it the menu leaves out
  readonly taskShown?: { project: string; name: string } | null;
  readonly actions?: PageActions;
}

// useCopied shows a copy button's done mark for 1.5 s
export function useCopied(): [boolean, () => void] {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => {
      setCopied(false);
    }, 1500);
    return () => {
      clearTimeout(timer);
    };
  }, [copied]);
  return [
    copied,
    () => {
      setCopied(true);
    },
  ];
}

// PageMenu is the menu "…" of a page, the same in a row and in a bar: every item is drawn, and what the bar or the row
// shows its CSS hides there, so that nothing is in both (ui.md); the frame's bar folds its own into it under 40rem
export function PageMenu({
  page: a,
  at,
  place,
  menuId,
  openers,
  onPick,
  onRemoved = () => undefined,
  taskShown = null,
  actions = pageActionStore,
}: PageMenuProps) {
  const arm = useArm(a.slug);
  const [copied, setCopied] = useCopied();
  const title = a.title ?? a.slug;
  const back = menuBack(menuId);
  const kind = removal(a);
  const name = a.task?.name ?? '';
  // What the bar or the row shows: in the frame's folded bar, the menu's
  const barShows = place === 'frame' ? 'narrow' : 'never';
  const linksShow = place === 'row' ? 'shown' : barShows;
  return (
    <MoreMenu
      id={menuId}
      label={t('ui_more_named', title)}
      size={place === 'row' ? 'row' : 'bar'}
      opens={place === 'frame' ? 'folds' : place === 'find' ? 'up' : 'down'}
    >
      <MenuItem
        icon={<StarIcon />}
        testid="fav-item"
        display={barShows}
        label={t(a.favorite ? 'ui_unfavorite_named' : 'ui_favorite_named', title)}
        onSelect={() => void actions.toggleFavorite(a)}
      >
        {t(a.favorite ? 'ui_unfavorite' : 'ui_favorite')}
      </MenuItem>
      <MenuItem
        icon={<UnreadIcon />}
        testid="mark-unread"
        // Archived, it is never unseen: marked unread, nothing would change
        hidden={[a.path, a.folder, a.news, a.archived].some(Boolean)}
        label={t('ui_mark_unread_named', title)}
        onSelect={() => void actions.markUnreadSaid(a.slug, unreadFocus)}
      >
        {t('ui_mark_unread')}
      </MenuItem>
      <MenuItem
        icon={<SelectIcon />}
        testid="select"
        hidden={place !== 'row' || !onPick}
        label={t('ui_select_named', title)}
        onSelect={onPick}
      >
        {t('ui_select')}
      </MenuItem>
      <MenuItem
        icon={<TagIcon />}
        testid="tag-item"
        label={t('ui_tag_change_named', title)}
        onSelect={() => {
          openers.onTags(a, back);
        }}
      >
        {t('ui_tags')}
      </MenuItem>
      <MenuItem
        icon={<ThemeIcon />}
        testid="theme-item"
        hidden={!a.themed}
        label={t('ui_theme_item_named', title)}
        onSelect={() => {
          openers.onTheme(a, back);
        }}
      >
        {t('ui_theme_item')}
      </MenuItem>
      <MenuItem
        icon={<PinIcon />}
        testid="pin-item"
        // Archived it leaves the pins, and comes back with its pin as it is given back
        hidden={Boolean(a.archived)}
        label={t(a.task?.pin ? 'ui_unpin_named' : 'ui_pin_named', title)}
        onSelect={() => void actions.togglePin(a)}
      >
        {t(a.task?.pin ? 'ui_unpin' : 'ui_pin')}
      </MenuItem>
      <MenuItem
        icon={<TaskSetIcon />}
        testid="task-set"
        label={t('ui_task_set_named', title)}
        onSelect={() => {
          openers.onTaskSet(a, back);
        }}
      >
        {t('ui_task_set')}
      </MenuItem>
      <MenuItem
        icon={<TaskOutIcon />}
        testid="task-out"
        hidden={!name}
        label={t('ui_task_out_named', title)}
        // Off its task at once, with the way back in the toast; from the keys, the focus on the way back
        onSelect={(e) =>
          void actions.takePageOff(a, {
            focus: e.detail === 0,
            prepare: (slug) => prepareTaskOut(slug, back),
          })
        }
      >
        {t('ui_task_out')}
      </MenuItem>
      <MenuItem
        icon={<ToIcon />}
        testid="task-open"
        hidden={!name || (name === taskShown?.name && a.taskProject === taskShown.project)}
        href={name ? taskHref({ project: a.taskProject, name }) : undefined}
      >
        {name ? t('ui_task_open', name) : ''}
      </MenuItem>
      <MenuItem
        icon={<EditIcon />}
        testid="edit"
        label={t('ui_edit_named', title)}
        onSelect={() => {
          openers.onEdit(a, (slug) => menuBack(place === 'row' ? rowMenuId(slug) : menuId)());
        }}
      >
        {t('ui_edit')}
      </MenuItem>
      <MenuItem
        icon={copied ? <DoneIcon /> : <CopyIcon />}
        testid="copy"
        hidden={!a.path}
        label={t('ui_copy_path_named', title)}
        title={copied ? t('ui_copied') : undefined}
        onSelect={() => {
          void actions.copyPath(a.path ?? '').then((ok) => {
            if (ok) setCopied();
          });
        }}
      >
        {t('ui_copy_path')}
      </MenuItem>
      <MenuItem
        icon={<OpenIcon />}
        testid="open"
        display={linksShow}
        hidden={!hasPage(a)}
        href={rawHref(at)}
        newTab
        label={t('ui_open_raw_named', title)}
      >
        {t('ui_open_raw')}
      </MenuItem>
      <MenuItem
        icon={<DownloadIcon />}
        testid="download"
        display={linksShow}
        hidden={!hasPage(a)}
        href={downloadHref(at)}
        download
        label={t('ui_download_named', title)}
      >
        {t('ui_download')}
      </MenuItem>
      <MenuItem
        icon={<ArchiveIcon />}
        testid="archive"
        display={barShows}
        label={t(a.archived ? 'ui_unarchive_named' : 'ui_archive_named', title)}
        onSelect={() =>
          void actions.setArchived(a, !a.archived, { prepare: prepareArchive, restoreFocus: restoreFocus(a.slug) })
        }
      >
        {t(a.archived ? 'ui_unarchive' : 'ui_archive')}
      </MenuItem>
      <MenuItem
        icon={<DeleteIcon />}
        testid="delete"
        tone="danger"
        // It stays for its second press
        keepsOpen
        armed={arm.armed}
        label={t(arm.armed ? `ui_${kind}_confirm_named` : `ui_${kind}_named`, title)}
        onBlur={arm.disarm}
        onSelect={() => {
          if (arm.press()) return;
          void actions.removePage(a, closedThen(menus, menuId, onRemoved));
        }}
      >
        {t(arm.armed ? `ui_${kind}_confirm` : `ui_${kind}`)}
      </MenuItem>
    </MoreMenu>
  );
}
