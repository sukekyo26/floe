import type { ReactNode } from 'react';

import { Button } from '@/components/button';
import { PlusIcon } from '@/components/icons';
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

const LINK = 'border-0 bg-transparent p-0 text-fg underline underline-offset-3';

function State({
  id,
  shown,
  children,
}: {
  readonly id: Empty;
  readonly shown: Empty | null;
  readonly children: ReactNode;
}) {
  return (
    <div
      id={id}
      data-testid="state"
      hidden={shown !== id}
      className="[&_strong]:text-base grid justify-items-center gap-2 px-6 py-14 text-center text-muted [&_p]:m-0 [&_p]:max-w-[34rem] [&_strong]:text-fg"
    >
      {children}
    </div>
  );
}

// ListEmpty is what the page list says in place of its rows (ui.html #empty … #nomatch): all six drawn, the one shown
// as emptyOf says, as the tests find each by its id
export function ListEmpty({ shown, archived, onAdd, onShowArchived, onClear }: ListEmptyProps) {
  return (
    <>
      <State id="empty" shown={shown}>
        <strong>{t('ui_empty_pages')}</strong>
        <p>{t('ui_empty_pages_hint')}</p>
        <p>
          <Button id="add-empty" onClick={onAdd}>
            <PlusIcon />
            {t('ui_add_link')}
          </Button>
        </p>
      </State>
      <State id="all-archived" shown={shown}>
        <strong>{t('ui_all_archived', archived)}</strong>
        <p>
          <button type="button" id="show-archived" onClick={onShowArchived} className={LINK}>
            {t('ui_show_archived')}
          </button>
        </p>
      </State>
      <State id="empty-new" shown={shown}>
        <strong>{t('ui_no_new')}</strong>
      </State>
      <State id="empty-favorites" shown={shown}>
        <strong>{t('ui_no_favorites')}</strong>
      </State>
      <State id="empty-archived" shown={shown}>
        <strong>{t('ui_none_archived')}</strong>
      </State>
      <State id="nomatch" shown={shown}>
        <strong>{t('ui_nomatch')}</strong>
        <p>
          <button type="button" id="clear" onClick={onClear} className={LINK}>
            {t('ui_clear_filters')}
          </button>
        </p>
      </State>
    </>
  );
}
