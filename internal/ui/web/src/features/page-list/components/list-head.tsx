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

// ListWhat is what the shelf narrows the page list to, with the shelf folded: the view and how many picks. Pressed, the
// shelf opens by onShow, the one way the columns open it (P5f's, which gives the focus to the button that folds it:
// this one hides). Where it stands, in the head of the list or of the page beside it, is the screen's
export function ListWhat({
  onShow,
  query = listQuery,
  columns = pageColumns,
}: Omit<ListHeadProps, 'pages'> & { readonly onShow: () => void }) {
  const { what } = narrowWords(useStore(query.query));
  return <ShelfWhat words={what} tint={null} hidden={useWhatHidden({ query, columns })} onShow={onShow} />;
}

// useWhatHidden tells ListWhat hidden: the shelf open, which says it itself, or nothing narrowed. The screen that places
// it reads it too, to leave the head of the page beside the list without it
export function useWhatHidden({
  query = listQuery,
  columns = pageColumns,
}: Omit<ListHeadProps, 'pages'> = {}): boolean {
  const { what } = narrowWords(useStore(query.query));
  return useStore(columns.columns).open || what === '';
}

// ListCount is the head of the list: how many new pages the view and the search leave
export function ListCount({ query = listQuery, pages = pagePages }: Omit<ListHeadProps, 'columns'>) {
  const state = useStore(query.query);
  const items = useStore(pages.state).entries;
  return (
    <span id="count" className="me-auto min-w-0 truncate">
      {t('ui_list_head', viewCounts(items, state, state.seen).new)}
    </span>
  );
}
