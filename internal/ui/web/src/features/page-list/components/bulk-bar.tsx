import { useMemo } from 'react';

import { Button } from '@/components/button';
import { bulkActs as makeBulkActs, type BulkActs, type BulkTags } from '@/features/page-list/stores/bulk-acts';
import { selection as pageSelection, type Selection } from '@/features/page-list/stores/selection';
import { bulkKind } from '@/features/page-list/utils/bulk';
import { useArm } from '@/hooks/use-arm';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';

export interface BulkBarProps {
  // onTags opens the tag modal for the selected
  readonly onTags: (target: BulkTags) => void;
  // LIMIT: the clearing's form is the user's to choose in the gallery (S425): its words (a short one, the long one its
  // name) and whether it stays at the bar's end. Once chosen, the other forms go
  readonly clearWords?: string;
  readonly clearSticky?: boolean;
  readonly selection?: Selection;
  readonly pages?: Pages;
  readonly query?: ListQuery;
  readonly acts?: BulkActs;
}

// The bar's own room for its buttons (ui.html .bulk .btn), given from here: none of it is the shared Button's. Its
// buttons but the clearing go while the delete waits (CSS 625), and narrow (the head's container bar) they close up
const BAR =
  'flex min-w-0 flex-1 items-center gap-1 overflow-x-auto overflow-y-hidden p-1 -m-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [&>button]:flex-none [&>button]:px-2 [&>button[data-armed]]:px-[0.8rem] has-[>[data-armed]]:[&>button:not([data-armed],#bulk-clear)]:hidden @max-[21rem]/bar:gap-[0.1rem] @max-[21rem]/bar:[&>button:not([data-armed])]:px-[0.2rem]';
const STICKY = '[&>#bulk-clear]:sticky [&>#bulk-clear]:right-0 [&>#bulk-clear]:bg-sel [&>#bulk-clear]:hover:bg-subtle';

// BulkBar is the bar of the selected over the list (ui.html #bulk, renderBulk): how many, select all, tags, archive,
// delete in two presses, and let them go. Its buttons are drawn while none is selected too, hidden with the bar, and
// stay the same elements as the selection changes. The delete waits for the selection as it is: changed, it lets go
export function BulkBar({
  onTags,
  clearWords,
  clearSticky = true,
  selection = pageSelection,
  pages = pagePages,
  query = listQuery,
  acts,
}: BulkBarProps) {
  const selected = useStore(selection.selected);
  const rev = useStore(selection.rev);
  const { view } = useStore(query.query);
  const items = useStore(pages.state).entries;
  const does = useMemo(() => acts ?? makeBulkActs({ selection, pages, query }), [acts, selection, pages, query]);
  const arm = useArm(String(rev));
  const n = selected.size;
  const kind = bulkKind(selected, items);
  const label = (words: string) => <span data-testid="label">{words}</span>;
  return (
    <section
      id="bulk"
      aria-label={t('ui_bulk_region')}
      hidden={n === 0}
      className={`${BAR} ${clearSticky ? STICKY : ''}`}
    >
      <span
        id="bulk-count"
        aria-live="polite"
        className="me-auto min-w-0 truncate ps-2 text-small font-[650] text-fg @max-[24rem]/bar:flex-none"
      >
        <span id="bulk-n">{t('ui_selected_n', n)}</span>
        <span className="@max-[24rem]/bar:hidden">{t('ui_selected_more')}</span>
      </span>
      <Button id="bulk-page" quiet label={t('ui_select_page')} onClick={does.selectAll}>
        {label(t('ui_select_all'))}
      </Button>
      <Button
        id="bulk-tag"
        quiet
        onClick={() => {
          onTags(does.tags());
        }}
      >
        {label(t('ui_tags'))}
      </Button>
      <Button id="bulk-archive" quiet onClick={() => void does.archive()}>
        {label(t(view === 'archived' ? 'ui_unarchive' : 'ui_archive'))}
      </Button>
      <Button
        id="bulk-delete"
        quiet
        tone="danger"
        armed={arm.armed}
        onClick={() => {
          if (arm.press()) return;
          void does.remove().then(() => {
            arm.disarm();
          });
        }}
        onBlur={arm.disarm}
      >
        {label(arm.armed ? t(`ui_${kind}_bulk_confirm`, n) : t(`ui_${kind}`))}
      </Button>
      <Button
        id="bulk-clear"
        quiet
        label={clearWords === undefined ? undefined : t('ui_clear_selection')}
        onClick={selection.clear}
      >
        {label(clearWords ?? t('ui_clear_selection'))}
      </Button>
    </section>
  );
}
