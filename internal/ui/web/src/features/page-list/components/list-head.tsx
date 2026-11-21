import { ShelfWhat } from '@/components/shelf-what';
import { narrowWords, viewCounts } from '@/features/page-list/utils/list-shelf';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { columns as pageColumns, type ColumnsStore } from '@/stores/columns';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';

export interface ListHeadProps {
  readonly query?: ListQuery;
  readonly pages?: Pages;
  readonly columns?: ColumnsStore;
}

// ListWhat is what the shelf narrows the page list to, with the shelf folded (ui.html drawViews, drawHead): the view and
// how many picks. Pressed, the shelf opens. Where it stands, in the head of the list or of the page beside it, is the
// screen's
export function ListWhat({ query = listQuery, columns = pageColumns }: Omit<ListHeadProps, 'pages'>) {
  const { what } = narrowWords(useStore(query.query));
  const { open } = useStore(columns.columns);
  return (
    <ShelfWhat
      words={what}
      tint={null}
      hidden={open || what === ''}
      onShow={() => {
        columns.set({ open: true });
        columns.keep();
      }}
    />
  );
}

// ListCount is the head of the list: how many new pages the view and the search leave (ui.html renderShelf #count)
export function ListCount({ query = listQuery, pages = pagePages }: Omit<ListHeadProps, 'columns'>) {
  const state = useStore(query.query);
  const items = useStore(pages.state).entries;
  return (
    <span id="count" className="me-auto min-w-0 truncate">
      {t('ui_list_head', viewCounts(items, state, state.seen).new)}
    </span>
  );
}
