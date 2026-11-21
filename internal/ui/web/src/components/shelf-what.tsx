import type { CSSProperties } from 'react';
import { t } from '@/lib/t';

import type { CssVars } from '@/utils/css-vars';

export interface ShelfWhatProps {
  // words say what the filters narrow the list to; tint is the task list's project's colour, null for none
  readonly words: string;
  readonly tint: string | null;
  // hidden is the caller's: with the shelf open, or no words (ui.html drawHead)
  readonly hidden: boolean;
  readonly onShow: () => void;
}

// ShelfWhat is what the filters narrow the list to, in the head of the list or of the page beside it, with the shelf
// folded (ui.html #shelf-what): pressed, the shelf opens. Wherever it stands it is gone from 70rem, where the shelf
// is a row over the list
export function ShelfWhat({ words, tint, hidden, onShow }: ShelfWhatProps) {
  const c: CssVars = tint === null ? {} : { '--c': tint };
  return (
    <button
      type="button"
      id="shelf-what"
      title={t('ui_shelf_show')}
      hidden={hidden}
      data-tint={tint === null ? undefined : 'true'}
      style={c as CSSProperties}
      onClick={onShow}
      className={`max-w-full flex-none truncate border-0 bg-transparent p-0 font-semibold text-fg underline underline-offset-3 wide:hidden ${tint === null ? '' : "inline-flex items-center gap-[0.3rem] rounded-full bg-(--tint) px-2 text-label leading-normal font-medium text-(--tint-ink) no-underline [--tint-ink:color-mix(in_srgb,var(--c)_50%,var(--color-fg))] [--tint:color-mix(in_srgb,var(--c)_16%,var(--color-panel))] before:size-1.75 before:flex-none before:rounded-full before:bg-(--c) before:content-['']"}`}
    >
      {words}
    </button>
  );
}
