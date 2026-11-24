import type { ReadMark } from '@/types/page';
import { t } from '@/lib/t';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pageActions, type PageActions } from '@/stores/page-actions';
import { pageNote, type PageNote } from '@/stores/page-note';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { reads as sharedReads, type Reads } from '@/stores/reads';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';

export interface PageReads {
  // opened is the page beside the list loaded anew: the next read told is of a page a person opened, the words of its
  // versions and its toast go, and a page marked unread holds through its versions until another page is open
  opened: (slug: string | undefined) => void;
  // served is a page telling the version it was served (pageServed): sent as read unless kept unread, and said, of the
  // page a person opened, once its answer clears the page's news while it is still the page shown. unreadFocus is
  // where the focus goes if the toast marking it unread had it
  served: (slug: string, mark: ReadMark, unreadFocus: () => void) => Promise<void>;
}

interface Deps {
  reads?: Reads;
  toasts?: Toasts;
  note?: PageNote;
  pages?: Pages;
  openPage?: OpenPage;
  actions?: PageActions;
}

// createPageReads is the read of a page shown: one per screen. A page whole is opened as the screen starts, so opening
// starts true
// LIMIT: of this tab alone: another tab with the page open reads it again as it draws itself anew. Tell the tabs if
// that comes to matter
// LIMIT: whole, a page gone back to by the browser (bfcache) tells nothing and stays unread, until opened again
export function createPageReads({
  reads = sharedReads,
  toasts = pageToasts,
  note = pageNote,
  pages = pagePages,
  openPage = pageOpen,
  actions = pageActions,
}: Deps = {}): PageReads {
  // Whether the next read told is of a page a person opened: the toast says so of no other
  let opening = true;
  let lastSlug: string | undefined;
  return {
    opened: (slug) => {
      // A page marked unread holds through its versions, until another is open
      if (slug !== lastSlug) reads.keptUnread.set(null);
      lastSlug = slug;
      note.clear();
      opening = true;
      for (const x of toasts.shown.get()) if (x.page) toasts.remove(x.id);
    },
    served: async (slug, mark, unreadFocus) => {
      const say = opening;
      opening = false;
      // Drawn anew at another version, what the words say and where they lead no longer agree
      const to = note.to();
      if (to !== null && to !== mark.n) note.clear();
      if (slug === reads.keptUnread.get()) return;
      const outcome = await reads.tellRead(slug, { read: mark, say });
      if (outcome.kind !== 'sent') return;
      // As the answer comes: still the page shown, and not marked unread meanwhile (TestUIUnreadKept 220). cleared is
      // of a read to be said alone, the one a second read took the place of too (S428)
      const shown = openPage.shownSlug() === slug;
      if (outcome.cleared && reads.keptUnread.get() !== slug && shown) {
        const a = pages.state.get().entries.find((x) => x.slug === slug);
        // Archived, it is never unseen: its way back to unread would change nothing
        if (!a?.archived) {
          toasts.toast(t('ui_read_done'), {
            ms: 8000,
            page: true,
            button: {
              label: t('ui_mark_unread'),
              name: t('ui_mark_unread_named', a?.title ?? slug),
              undo: false,
              holds: false,
              onPress: () => actions.markUnreadSaid(slug, unreadFocus),
            },
          });
        }
        // Not waited for: it holds up nothing
        void note.show(slug, mark.n);
      }
    },
  };
}

export const pageReads = createPageReads();
