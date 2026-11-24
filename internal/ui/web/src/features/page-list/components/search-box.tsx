import { useState } from 'react';

import { SearchIcon } from '@/components/icons';
import { searchKey } from '@/features/page-list/utils/search-key';
import { showResults } from '@/features/page-list/stores/show-results';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { columns as pageColumns, type ColumnsStore } from '@/stores/columns';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { composing } from '@/utils/keys';

export interface SearchBoxProps {
  readonly hidden?: boolean;
  readonly query?: ListQuery;
  readonly columns?: ColumnsStore;
  // go is where Enter goes with the words, on a screen of no list to narrow (a frame whose page is not found): the
  // words are the field's own, and the list's query and columns are left as they are
  readonly go?: (words: string) => void;
}

// SearchBox is the list's search in the bar: up to 34rem in the bar's middle, each letter narrows the list, Escape
// empties it and leaves it. Under 40rem it is its icon until it is reached or holds words
export function SearchBox({ hidden = false, query = listQuery, columns = pageColumns, go }: SearchBoxProps) {
  const { q } = useStore(query.query);
  const [own, setOwn] = useState('');
  const words = (value: string) => {
    if (go) {
      setOwn(value);
      return;
    }
    query.setWords(value);
    showResults(columns);
  };
  return (
    <label
      id="search"
      hidden={hidden}
      className="group/search relative mx-auto flex-[0_1_34rem] narrow:mx-0 narrow:flex-[0_0_36px] narrow:transition-[flex-basis] narrow:duration-150 narrow:focus-within:flex-[1_1_auto] narrow:has-[input:not(:placeholder-shown)]:flex-[1_1_auto] [&>svg]:pointer-events-none [&>svg]:absolute [&>svg]:top-1/2 [&>svg]:left-3 [&>svg]:size-[15px] [&>svg]:-translate-y-1/2 [&>svg]:text-faint"
    >
      <SearchIcon />
      <input
        type="search"
        id="q"
        value={go ? own : q}
        placeholder={t('ui_search_placeholder')}
        aria-label={t('ui_search')}
        autoComplete="off"
        onChange={(e) => {
          words(e.currentTarget.value);
        }}
        onKeyDown={(e) => {
          if (composing(e.nativeEvent)) return;
          const field = e.currentTarget;
          searchKey(e.key, own, {
            go,
            leave: () => {
              words('');
              field.blur();
            },
          });
        }}
        className="peer h-9 w-full min-w-0 rounded-full border border-line bg-panel ps-[2.2rem] pe-10 font-normal text-fg shadow-sm placeholder:text-faint narrow:pe-2 narrow:not-focus:placeholder-shown:text-transparent narrow:not-focus:placeholder-shown:placeholder:text-transparent [&::-webkit-search-cancel-button]:hidden"
      />
      <kbd
        aria-hidden="true"
        className="absolute top-1/2 right-[0.6rem] -translate-y-1/2 rounded-small border border-line bg-subtle px-[0.4rem] py-[0.2rem] font-mono text-label leading-none text-faint peer-focus:hidden narrow:hidden"
      >
        /
      </kbd>
    </label>
  );
}
