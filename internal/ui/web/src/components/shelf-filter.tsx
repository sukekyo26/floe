import type { CSSProperties } from 'react';

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
  readonly onOpen: () => void;
}

// NarrowFilter is the way to the rest of the shelf on a narrow screen (ui.html #narrow-filter), at the end of the row
// of views, lit while picks narrow the list; on a phone its count stands for its words
export function NarrowFilter({ words, n, tint, onOpen }: NarrowFilterProps) {
  const c: CssVars = tint === null ? {} : { '--c': tint };
  return (
    <button
      type="button"
      id="narrow-filter"
      data-narrowed={n > 0 ? 'true' : undefined}
      data-tint={tint === null ? undefined : 'true'}
      style={c as CSSProperties}
      onClick={onOpen}
      className={`${SHELF_ROW} ${VIEW_PILL} ${SHELF_MARK} hidden text-fg [--tint-ink:color-mix(in_srgb,var(--c)_50%,var(--color-fg))] [--tint:color-mix(in_srgb,var(--c)_16%,var(--color-panel))] data-tint:border-transparent data-tint:bg-(--tint) data-tint:font-medium data-tint:text-(--tint-ink) narrow:ms-auto wide:inline-flex wide:max-w-full wide:min-w-0 wide:data-narrowed:border-ink wide:data-narrowed:bg-ink wide:data-narrowed:text-on-ink wide:data-narrowed:[&_svg]:text-on-ink`}
    >
      <FilterIcon />
      <span
        id="narrow-filter-name"
        className={`tiny:sr-only ${tint === null ? '' : "inline-flex items-center gap-[0.3rem] before:size-1.75 before:flex-none before:rounded-full before:bg-(--c) before:content-['']"}`}
      >
        {words}
      </span>
      <span id="narrow-filter-n" aria-hidden="true" className="hidden tiny:inline">
        {n > 0 ? n : ''}
      </span>
    </button>
  );
}
