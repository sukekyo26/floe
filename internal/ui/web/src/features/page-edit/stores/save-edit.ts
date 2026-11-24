import { t } from '@/lib/t';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';
import { pageWrites, type PageWrites } from '@/stores/page-writes';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { Page } from '@/types/page';
import { pageHref } from '@/utils/page-urls';

// The fields of the edit modal, as typed
export interface EditValues {
  title: string;
  slug: string;
  project: string;
  description: string;
}

// What came of a save: ran is false for a second press while held, which leaves the modal as it is; reason is a
// refusal said in the modal, with field the one to take the focus ('' for none); away tells that the page whole moved to
// its new name, and the modal is left to go with it; slug is the page's now, for the focus to go back to its menu
export interface EditSaved {
  ran: boolean;
  reason: string;
  field: '' | 'slug' | 'project';
  slug: string;
  away: boolean;
}

interface Deps {
  writes?: PageWrites;
  pages?: Pages;
  toasts?: Toasts;
  openPage?: OpenPage;
  order?: Order;
  go?: (href: string) => void;
}

// edited are the fields changed, trimmed: the store trims the title and the description, the slug's pattern refuses a
// space and ProjectKey trims the project, so a change of spaces alone is none
export const edited = (a: Page, values: EditValues): Partial<EditValues> =>
  Object.fromEntries(
    (Object.keys(values) as (keyof EditValues)[])
      .map((key) => [key, values[key].trim()] as const)
      .filter(([key, value]) => value !== (a[key] ?? '')),
  );

// createSaveEdit saves a page's edit: held, only the fields changed are sent; renamed whole, the page goes to its new
// name (the rows that opened it keep it), renamed beside the list, it stays open under its new one; then it is said,
// and the list read, before the modal closes and gives the focus to the row's menu as drawn
export function createSaveEdit({
  writes = pageWrites,
  pages = pagePages,
  toasts = pageToasts,
  openPage = pageOpen,
  order = pageOrder,
  go = (href) => {
    window.location.replace(href);
  },
}: Deps = {}): (
  a: Page,
  values: EditValues,
  whileHeld: (write: () => Promise<void>) => Promise<boolean>,
) => Promise<EditSaved> {
  return async (a, values, whileHeld) => {
    const changes = edited(a, values);
    const saved: EditSaved = { ran: false, reason: '', field: '', slug: a.slug, away: false };
    saved.ran = await whileHeld(async () => {
      let offTask = '';
      saved.reason = await writes.writePage(
        a.slug,
        { method: 'PATCH', headers: { 'content-type': 'application/json' }, body: JSON.stringify(changes) },
        {
          answer: (got) => {
            const off = (got as { offTask?: unknown } | null)?.offTask;
            offTask = typeof off === 'string' ? off : '';
          },
        },
      );
      if (saved.reason) return;
      if (changes.slug !== undefined) {
        saved.slug = changes.slug;
        const whole = openPage.frameAt();
        // This frame's URL is the old name's, which is gone now
        if (whole?.slug === a.slug) {
          order.renameKept(a.slug, changes.slug);
          go(pageHref({ slug: changes.slug, v: whole.v }));
          saved.away = true;
          return;
        }
        const picked = openPage.picked.get();
        if (picked?.slug === a.slug) openPage.picked.set({ ...picked, slug: changes.slug });
      }
      // A page moved to another project is off its task, which is of its project
      toasts.toast(offTask ? t('ui_saved_off_task', offTask) : t('ui_saved'));
      await pages.load();
    });
    // LIMIT: the server refuses in words, not keys, so which field it was is not known here: the field is chosen from
    // what was sent, the slug's before the project's. Put it on the field refused once a refusal names its key
    if (saved.reason) saved.field = 'slug' in changes ? 'slug' : 'project' in changes ? 'project' : '';
    return saved;
  };
}

export const saveEdit = createSaveEdit();
