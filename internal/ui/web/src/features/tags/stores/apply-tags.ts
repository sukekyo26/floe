import { toSend } from '@/features/tags/utils/tag-marks';
import { t } from '@/lib/t';
import { actOn as pageActOn, type ActOn, type ActWords } from '@/stores/act-on';
import { pageWrites, type PageWrites } from '@/stores/page-writes';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';

export interface TagApply {
  slugs: string[];
  add: string[];
  remove: string[];
  words: ActWords;
  // send writes the pages (actOn, or the selected's, which keeps those that failed selected: P5)
  send?: ActOn;
  // after tidies what opened the modal, given the slugs that will be sent, before they are (P5 unselects the rest)
  after?: (next: Set<string>) => void;
  whileHeld: (write: () => Promise<void>) => Promise<boolean>;
  close: () => void;
}

interface Deps {
  pages?: Pages;
  writes?: PageWrites;
  toasts?: Toasts;
  actOn?: ActOn;
}

// createApplyTags writes the tags chosen: held, the list is read again first, so that a page removed since is not sent
// rather than be sent and not found; then the pages these change are sent, and the list read after (actOn). The modal
// closes once the list is read, so that the focus goes back to a row's menu as drawn anew, and closes whatever throws;
// a second press while held does not run and leaves it open
export function createApplyTags({
  pages = pagePages,
  writes = pageWrites,
  toasts = pageToasts,
  actOn = pageActOn,
}: Deps = {}): (apply: TagApply) => Promise<void> {
  return async ({ slugs, add, remove, words, send = actOn, after, whileHeld, close }) => {
    let ran = true;
    try {
      ran = await whileHeld(async () => {
        await pages.load();
        const next = toSend(pages.state.get().entries, slugs, add, remove);
        after?.(new Set(next));
        if (next.length === 0) toasts.toast(t('ui_tag_bulk_none'));
        else await send((slug) => writes.patchPage(slug, { tags: { add, remove } }), words, next);
      });
    } finally {
      if (ran) close();
    }
  };
}

export const applyTags = createApplyTags();
