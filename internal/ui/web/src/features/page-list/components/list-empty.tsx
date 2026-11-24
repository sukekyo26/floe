import { Button } from '@/components/button';
import { EmptyState } from '@/components/empty-state';
import { PlusIcon } from '@/components/icons';
import { LINK_LOOK } from '@/components/text-link';
import type { Empty } from '@/features/page-list/utils/list-empty';
import { t } from '@/lib/t';

export interface ListEmptyProps {
  // shown is the words that stand for the rows, null for none
  readonly shown: Empty | null;
  // archived is how many pages are archived, which all-archived counts
  readonly archived: number;
  readonly onAdd: () => void;
  readonly onShowArchived: () => void;
  readonly onClear: () => void;
}

const LINK = `border-0 bg-transparent p-0 text-fg ${LINK_LOOK}`;

// ListEmpty is what the page list says in place of its rows: all six drawn, the one shown as emptyOf says, as the tests
// find each by its id
export function ListEmpty({ shown, archived, onAdd, onShowArchived, onClear }: ListEmptyProps) {
  return (
    <>
      <EmptyState id="empty" hidden={shown !== 'empty'}>
        <strong>{t('ui_empty_pages')}</strong>
        <p>{t('ui_empty_pages_hint')}</p>
        <p>
          <Button id="add-empty" adds onClick={onAdd}>
            <PlusIcon />
            {t('ui_add_link')}
          </Button>
        </p>
      </EmptyState>
      <EmptyState id="all-archived" hidden={shown !== 'all-archived'}>
        <strong>{t('ui_all_archived', archived)}</strong>
        <p>
          <button type="button" id="show-archived" onClick={onShowArchived} className={LINK}>
            {t('ui_show_archived')}
          </button>
        </p>
      </EmptyState>
      <EmptyState id="empty-new" hidden={shown !== 'empty-new'}>
        <strong>{t('ui_no_new')}</strong>
      </EmptyState>
      <EmptyState id="empty-favorites" hidden={shown !== 'empty-favorites'}>
        <strong>{t('ui_no_favorites')}</strong>
      </EmptyState>
      <EmptyState id="empty-archived" hidden={shown !== 'empty-archived'}>
        <strong>{t('ui_none_archived')}</strong>
      </EmptyState>
      <EmptyState id="nomatch" hidden={shown !== 'nomatch'}>
        <strong>{t('ui_nomatch')}</strong>
        <p>
          <button type="button" id="clear" onClick={onClear} className={LINK}>
            {t('ui_clear_filters')}
          </button>
        </p>
      </EmptyState>
    </>
  );
}
