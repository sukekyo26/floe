import type { ReactNode } from 'react';

import { SHELF_MARK, SHELF_N, SHELF_PRESSED, SHELF_ROW } from '@/components/shelf-chip';

// Up to 70rem the views are a row of pills over the list, with the way to the filters; pressed, filled with ink.
// Shared with the button to the filters, which stands in that row
export const VIEW_PILL =
  'wide:w-auto wide:flex-none wide:gap-[0.3rem] wide:rounded-full wide:border wide:border-line wide:bg-panel wide:px-[0.55rem] wide:py-[0.2rem] wide:whitespace-nowrap wide:hover:bg-panel wide:aria-pressed:border-ink wide:aria-pressed:bg-ink wide:aria-pressed:text-on-ink wide:aria-pressed:shadow-none';

export interface ViewButtonProps {
  readonly view: string;
  // mark is its dot, ring or icon; label its words; n how many it holds
  readonly mark: ReactNode;
  readonly label: string;
  readonly n: number;
  readonly lit: boolean;
  readonly onPress: () => void;
  // labelFrom hides the words for sight alone from a width (the row of pills: the star's, the archive's from 40rem);
  // countFolds hides the count in the row of pills
  readonly labelFrom?: 'wide' | 'narrow';
  readonly countFolds?: boolean;
  // markColor is its mark's colour, muted but the star's
  readonly markColor?: string;
}

const SR_ONLY = { wide: 'wide:sr-only', narrow: 'narrow:sr-only' } as const;

// ViewButton is a view of the shelf: lit when it is the list shown, with its count. One with none left stays where it
// is, quieter
export function ViewButton({
  view,
  mark,
  label,
  n,
  lit,
  onPress,
  labelFrom,
  countFolds = false,
  markColor = SHELF_MARK,
}: ViewButtonProps) {
  return (
    <button
      type="button"
      data-view={view}
      aria-pressed={lit}
      onClick={onPress}
      className={`group flex ${SHELF_ROW} ${SHELF_PRESSED} ${VIEW_PILL} ${markColor} text-fg`}
    >
      {mark}
      <span
        data-testid="label"
        className={`${n === 0 && !lit ? 'text-muted' : ''} ${labelFrom ? SR_ONLY[labelFrom] : ''}`}
      >
        {label}
      </span>
      <span
        data-testid="n"
        className={`${SHELF_N} wide:ms-0 wide:group-aria-pressed:text-on-ink ${countFolds ? 'wide:hidden' : ''}`}
      >
        {n}
      </span>
    </button>
  );
}
