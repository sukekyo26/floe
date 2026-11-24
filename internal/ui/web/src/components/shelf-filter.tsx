import type { CSSProperties } from 'react';

import { TINT_DOT, TINT_VARS } from '@/components/tint';
import { FilterIcon } from '@/components/icons';
import { SHELF_MARK, SHELF_ROW } from '@/components/shelf-chip';
import { VIEW_PILL } from '@/components/shelf-views';
import type { CssVars } from '@/utils/css-vars';

export interface NarrowFilterProps {
  // words say what narrows the list (the page list: how many picks; the task list: its project), n how many picks
  readonly words: string;
  readonly n: number;
  // tint is the project's colour the task list paints it in, null for none
  readonly tint: string | null;
  // named: the words always show, cut short, and no count (the task list, which always shows one project)
  readonly named?: boolean;
  readonly onOpen: () => void;
}

// NarrowFilter is the way to the rest of the shelf on a narrow screen, at the end of the row of views, lit while picks
// narrow the list; on a phone its count stands for its words
export function NarrowFilter({ words, n, tint, named = false, onOpen }: NarrowFilterProps) {
  const dot =
    tint === null
      ? ''
      : named
        ? `before:me-[0.3rem] before:inline-block ${TINT_DOT}`
        : `inline-flex items-center gap-[0.3rem] ${TINT_DOT}`;
  const c: CssVars = tint === null ? {} : { '--c': tint };
  return (
    <button
      type="button"
      id="narrow-filter"
      data-narrowed={n > 0 ? 'true' : undefined}
      data-tint={tint === null ? undefined : 'true'}
      style={c as CSSProperties}
      onClick={onOpen}
      className={`${SHELF_ROW} ${VIEW_PILL} ${SHELF_MARK} hidden text-fg ${TINT_VARS} data-tint:border-transparent data-tint:bg-(--tint) data-tint:font-medium data-tint:text-(--tint-ink) narrow:ms-auto ${named ? 'narrow:shrink!' : ''} wide:inline-flex wide:max-w-full wide:min-w-0 wide:data-narrowed:border-ink wide:data-narrowed:bg-ink wide:data-narrowed:text-on-ink wide:data-narrowed:[&_svg]:text-on-ink`}
    >
      <FilterIcon />
      <span
        id="narrow-filter-name"
        className={`${named ? 'truncate narrow:max-w-[4.5rem] narrow:min-w-[2em]' : 'tiny:sr-only'} ${dot}`}
      >
        {words}
      </span>
      <span id="narrow-filter-n" aria-hidden="true" className={named ? 'hidden' : 'hidden tiny:inline'}>
        {n > 0 ? n : ''}
      </span>
    </button>
  );
}
