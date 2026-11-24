import { getBoot, type Boot } from '@/config/boot';
import { t } from '@/lib/t';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import { order as pageOrder, type Order } from '@/stores/order';
import { pageWrites as pageWriteStore, type PageWrites } from '@/stores/page-writes';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { pressOnce, type PressOnce } from '@/stores/press-once';
import { reads as pageReads, type Reads } from '@/stores/reads';
import { taskWrites as pageTaskWrites, type TaskWrites } from '@/stores/tasks';
import { buttonOf, toasts as pageToasts, type Toasts } from '@/stores/toasts';
import { undoToast as pageUndoToast, type UndoToast } from '@/stores/undo-toast';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pageNote as pagePageNote, type PageNote } from '@/stores/page-note';
import type { Page } from '@/types/page';
import { removal } from '@/utils/list-view';

// Where the focus goes after an archive, taken before the list is read: Escape on its way back
export interface ArchiveFocus {
  escape: () => void;
}

// Where the focus goes after a page is taken off its task, taken before the write: once it is put back (done), and on
// Escape on its way back (away)
export interface TaskOutFocus {
  done: () => void;
  away: () => void;
}

// What the caller gives an archive: whether it came from the keys (its way back takes the focus), where Escape on its
// way back leaves the focus, taken before the row goes, and where the focus goes once it is restored from its way back
export interface ArchiveCall {
  focus?: boolean;
  prepare: (slug: string) => ArchiveFocus;
  restoreFocus: () => void;
}

export type TakenOff = { reason: string; toast?: undefined } | { reason: ''; toast: number };

export interface PageActions {
  toggleFavorite: (a: Page) => Promise<void>;
  setArchived: (a: Page, to: boolean, call: ArchiveCall) => Promise<void>;
  // removePage deletes the page on the second press of its button; removed tells the caller it is gone (the selection)
  removePage: (a: Page, removed: () => void) => Promise<void>;
  togglePin: (a: Page) => Promise<void>;
  takePageOff: (a: Page, call: { focus?: boolean; prepare: (slug: string) => TaskOutFocus }) => Promise<void>;
  // takeOff takes the page off its task and gives the toast its way back, saying and reading nothing: the reason it was
  // refused, or the toast's id (Set the task's way off, which says it in its modal and reads held)
  takeOff: (a: Page, prepare: (slug: string) => TaskOutFocus, call?: { focus?: boolean }) => Promise<TakenOff>;
  // copyPath writes path to the clipboard: true once written
  copyPath: (path: string) => Promise<boolean>;
  // markUnreadSaid marks slug unread and says so; unreadFocus is where the focus goes if the toast it takes the place of
  // had it
  markUnreadSaid: (slug: string, unreadFocus: () => void) => Promise<void>;
}

interface Deps {
  writes?: PageWrites;
  tasks?: TaskWrites;
  pages?: Pages;
  reads?: Reads;
  notice?: Notice;
  toasts?: Toasts;
  undoToast?: UndoToast;
  order?: Order;
  openPage?: OpenPage;
  note?: PageNote;
  boot?: () => Boot;
  // go leaves for href (a page whole deleted goes back where it came from)
  go?: (href: string) => void;
  press?: PressOnce;
  // write puts text on the clipboard
  write?: (text: string) => Promise<void>;
}

const titleOf = (a: Page) => a.title ?? a.slug;

// createPageActions is what the bar, a row and their menus do to a page: each writes, says it, and reads the list after
// the answer, the toast shown before that read
// LIMIT: a page's write has no rev, so a way back writes over what was written to the page since
export function createPageActions({
  writes = pageWriteStore,
  tasks = pageTaskWrites,
  pages = pagePages,
  reads = pageReads,
  notice = pageNotice,
  toasts = pageToasts,
  undoToast = pageUndoToast,
  order = pageOrder,
  openPage = pageOpen,
  note = pagePageNote,
  boot = getBoot,
  go = (href) => {
    window.location.assign(href);
  },
  press = pressOnce,
  write = (text) => navigator.clipboard.writeText(text),
}: Deps = {}): PageActions {
  // The row gone from under the pointer, or moved, or left dimmed on a task's page: the way back is at hand, long
  // enough to read and reach
  function archivedDone(a: Page, call: ArchiveCall) {
    const where = call.prepare(a.slug);
    const id = toasts.toast(t('ui_archive_done', titleOf(a)), {
      ms: 8000,
      focus: call.focus ?? false,
      button: {
        label: t('ui_unarchive'),
        name: t('ui_unarchive_named', titleOf(a)),
        slug: a.slug,
        undo: true,
        // Not held: given back, its own toast goes, the focus on what came back if it was there
        holds: false,
        onPress: () => setArchived(a, false, call),
        onEscape: () => {
          toasts.remove(id);
          where.escape();
        },
      },
    });
  }

  // Pressed again before the list has it, a is as it was and the press would ask the same: it does nothing
  // LIMIT: archived again from the keys before the read after a restore by the keys, where the row stays (a task's page),
  // the focus goes to the row, off the second way back. Give it only while nothing has the focus if that comes to trouble
  // someone
  async function setArchived(a: Page, to: boolean, call: ArchiveCall) {
    await press(`archive page ${a.slug}`, String(to), () => archiveNow(a, to, call));
  }

  async function archiveNow(a: Page, to: boolean, call: ArchiveCall) {
    const reason = await writes.patchPage(a.slug, { archived: to });
    if (reason) {
      notice.say(t(to ? 'ui_archive_failed' : 'ui_unarchive_failed', titleOf(a), reason));
      return;
    }
    let hadFocus = false;
    if (to) archivedDone(a, call);
    else {
      // Given back, its ways back go, read before they do: the focus they held goes with them
      const ways = toasts.shown.get().filter((x) => buttonOf(x)?.undo && buttonOf(x)?.slug === a.slug);
      hadFocus = ways.some((x) => x.id === toasts.focused.get());
      for (const x of ways) toasts.remove(x.id);
      toasts.toast(t('ui_unarchive_done', titleOf(a)));
    }
    await pages.load();
    if (hadFocus) call.restoreFocus();
  }

  // LIMIT: put on another task since, the page is moved back unasked, the toast not saying the task it left. Ask and
  // say it as Set the task does if that comes to trouble someone
  // LIMIT: a write of the screen's own since, a page put on the task say, refuses it too. Take back the screen's own
  // writes too if that comes to trouble someone
  async function takeOff(
    a: Page,
    prepare: (slug: string) => TaskOutFocus,
    { focus = false }: { focus?: boolean } = {},
  ): Promise<TakenOff> {
    const name = a.task?.name ?? '';
    const category = a.task?.category ?? '';
    const where = prepare(a.slug);
    const reason = await tasks.changePageTask(a.slug, { task: '', category: '' });
    if (reason) return { reason };
    // The way back is of the task as the toast shows it: its rev as read now, or new when it has no row. A page
    // whole's load reads no tasks
    if (boot().screen === 'frame') await pages.rereadTasks();
    const known = Object.hasOwn(pages.state.get().tasks.tasks[a.taskProject] ?? {}, name);
    const seen = known ? { new: false, rev: pages.readRev(a.taskProject, name) } : { new: true };
    const toast = undoToast(
      t('ui_task_out_done', titleOf(a), name),
      {
        undo: async () => {
          const res = await tasks.putPageOn(a, name, seen, category);
          return { ok: res.ok, text: res.status === 409 ? res.stale : res.text };
        },
        done: where.done,
        away: where.away,
      },
      { focus },
    );
    return { reason: '', toast };
  }

  return {
    toggleFavorite: async (a) => {
      const reason = await writes.patchPage(a.slug, { favorite: !a.favorite });
      if (reason) {
        notice.say(t('ui_favorite_failed', reason));
        return;
      }
      toasts.toast(t(a.favorite ? 'ui_unfavorited' : 'ui_favorited', titleOf(a)));
      await pages.load();
    },
    setArchived,
    removePage: async (a, removed) => {
      const kind = removal(a);
      const reason = await writes.deletePage(a.slug);
      if (reason) {
        notice.say(t(kind === 'unlink' ? 'ui_unlink_failed' : 'ui_delete_failed', titleOf(a), reason));
        return;
      }
      removed();
      if (a.slug === boot().framed) {
        go(order.keptBack());
        return;
      }
      toasts.toast(t(kind === 'unlink' ? 'ui_unlink_done' : 'ui_delete_done', titleOf(a)));
      // Read, not taken out of the list here: a list changed here would be drawn anew by the next read
      await pages.load();
    },
    togglePin: async (a) => {
      const reason = await tasks.changePageTask(a.slug, { pin: !a.task?.pin });
      if (reason) {
        notice.say(t('ui_save_failed', reason));
        return;
      }
      toasts.toast(t(a.task?.pin ? 'ui_unpinned' : 'ui_pinned', titleOf(a)));
      await pages.load();
    },
    takePageOff: async (a, { focus = false, prepare }) => {
      const off = await takeOff(a, prepare, { focus });
      if (off.reason) {
        notice.say(t('ui_save_failed', off.reason));
        return;
      }
      await pages.load();
    },
    takeOff,
    copyPath: async (path) => {
      try {
        await write(path);
      } catch (err) {
        notice.say(t('ui_copy_failed', err instanceof Error ? err.message : String(err)));
        return false;
      }
      toasts.toast(t('ui_path_copied'));
      return true;
    },
    markUnreadSaid: async (slug, unreadFocus) => {
      const marked = reads.markUnread(slug);
      if (!marked) return;
      // Of the page shown, the words of its versions go with what was read
      if (slug === openPage.shownSlug()) note.clear();
      const outcome = await marked;
      if (outcome.kind !== 'sent') return;
      // Of the page shown as the answer comes it takes the place of its toast; of another, it leaves the toast of the
      // one shown to press. Whether that toast had the focus is read before it goes
      const page = slug === openPage.shownSlug();
      const had = page && toasts.shown.get().some((x) => x.page && x.id === toasts.focused.get());
      toasts.toast(t('ui_unread_done'), { ms: 5000, page });
      if (had) unreadFocus();
    },
  };
}

export const pageActions = createPageActions();
