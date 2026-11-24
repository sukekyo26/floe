import { selection as pageSelection, type Selection } from '@/features/page-list/stores/selection';
import { bulkKind } from '@/features/page-list/utils/bulk';
import { t } from '@/lib/t';
import { actOn as pageActOn, type ActOn, type ActWords } from '@/stores/act-on';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pageWrites as pageWriteStore, type PageWrites } from '@/stores/page-writes';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { pressOnce, type PressOnce } from '@/stores/press-once';
import { arrange } from '@/utils/list-view';

// The tag modal's target of the selected, of the shape the tags feature takes (TagTarget), which the app hands it: a
// feature imports no other
export interface BulkTags {
  slugs: string[];
  title: (n: number) => string;
  done: (n: number) => string;
  failed: (n: number, why: string) => string;
  send: ActOn;
  after: (next: Set<string>) => void;
  back: null;
}

export interface BulkActs {
  // selectAll selects every entry the list draws: folders too, a tree's rows none
  selectAll: () => void;
  // archive archives the selected, or restores them on the view of the archived
  archive: () => Promise<void>;
  // remove deletes the selected, or unlinks them where every one is a link: its words of before the write. It tells
  // whether it ran: pressed again while the same go, it does not
  remove: () => Promise<boolean>;
  // tags is the tag modal's target of the selected
  tags: () => BulkTags;
}

interface Deps {
  selection?: Selection;
  actOn?: ActOn;
  writes?: PageWrites;
  pages?: Pages;
  query?: ListQuery;
  press?: PressOnce;
}

// bulkActs is what the bar of the selected does to them: those written leave the selection, those that failed stay in
// it
export function bulkActs({
  selection = pageSelection,
  actOn = pageActOn,
  writes = pageWriteStore,
  pages = pagePages,
  query = listQuery,
  press = pressOnce,
}: Deps = {}): BulkActs {
  const send: ActOn = (act, words, slugs, settled) =>
    actOn(act, words, slugs, (reasons) => {
      settled?.(reasons);
      const written = new Set(slugs.filter((_, i) => !reasons[i]));
      selection.keep(new Set([...selection.selected.get()].filter((slug) => !written.has(slug))));
    });
  const selected = () => [...selection.selected.get()];
  return {
    selectAll: () => {
      const now = query.query.get();
      selection.add(arrange(pages.state.get().entries, now, now.seen).map((a) => a.slug));
    },
    archive: async () => {
      const to = query.query.get().view !== 'archived';
      const words: ActWords = {
        done: (n) => t(to ? 'ui_archive_bulk_done' : 'ui_unarchive_bulk_done', n),
        failed: (n, why) => t(to ? 'ui_archive_bulk_failed' : 'ui_unarchive_bulk_failed', n, why),
      };
      const slugs = selected();
      await press('archive selected', [String(to), ...slugs].join('\n'), () =>
        send((slug) => writes.patchPage(slug, { archived: to }), words, slugs),
      );
    },
    remove: async () => {
      const kind = bulkKind(selection.selected.get(), pages.state.get().entries);
      const words: ActWords = {
        done: (n) => t(`ui_${kind}_bulk_done`, n),
        failed: (n, why) => t(`ui_${kind}_bulk_failed`, n, why),
      };
      const slugs = selected();
      const ran = await press('delete selected', slugs.join('\n'), async () => {
        await send((slug) => writes.deletePage(slug), words, slugs);
        return true;
      });
      return ran ?? false;
    },
    tags: () => ({
      slugs: selected(),
      title: (n) => t('ui_tag_bulk_title', n),
      done: (n) => t('ui_tag_bulk_done', n),
      failed: (n, why) => t('ui_tag_bulk_failed', n, why),
      send,
      // The rest leave the selection unsent
      after: (next) => {
        selection.keep(next);
      },
      back: null,
    }),
  };
}
