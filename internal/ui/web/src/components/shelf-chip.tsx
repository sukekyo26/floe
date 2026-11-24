import type { CSSProperties, ReactNode } from 'react';

import type { CssVars } from '@/utils/css-vars';

// A row of the shelf: a view, a chip or an add button, the width of the shelf, its mark in one column so that the words
// of every row start at one x (their colours and how they show are each one's own). Pressed, it stands out filled
export const SHELF_ROW =
  'w-full items-center gap-[0.55rem] rounded-menu border-0 bg-transparent px-[0.65rem] py-[0.42rem] text-start font-medium wrap-anywhere hover:bg-subtle [&_svg]:size-shelf-mark [&_svg]:flex-none';
// The marks' colour, where a row has none of its own (the star's)
export const SHELF_MARK = '[&_svg]:text-muted';
export const SHELF_PRESSED =
  'aria-pressed:bg-panel aria-pressed:font-bold aria-pressed:shadow-[0_1px_0_var(--color-line)]';
// A row's count, of mono digits that line up
export const SHELF_N = 'ms-auto font-mono text-label leading-none text-muted tabular-nums';

export interface ChipProps {
  readonly label: string;
  readonly n: number;
  readonly pressed: boolean;
  readonly onPick: () => void;
  // mark goes before the words (a kind's); dot is a project's colour, drawn as its dot
  readonly mark?: ReactNode;
  readonly dot?: string;
  // data names the chip by what it picks (data-kind, data-project, data-template, data-tag)
  readonly data: Readonly<Record<`data-${string}`, string>>;
}

// Chip is a button of a part of the shelf, with its count: pressed when it is the one picked. Its words are a name the
// API was sent, on one line, and whole in its title. One with none left stays where it is, quieter
export function Chip({ label, n, pressed, onPick, mark, dot, data }: ChipProps) {
  const tint: CssVars = dot === undefined ? {} : { '--c': dot };
  return (
    <button
      type="button"
      data-testid="chip"
      title={label}
      aria-pressed={pressed}
      onClick={onPick}
      style={tint as CSSProperties}
      className={`flex ${SHELF_ROW} ${SHELF_PRESSED} ${SHELF_MARK} text-fg ${dot === undefined ? '' : "before:mx-[calc((var(--spacing-shelf-mark)-8px)/2)] before:size-2 before:flex-none before:rounded-full before:bg-(--c) before:content-['']"}`}
      {...data}
    >
      {mark}
      <span data-testid="name" className={`min-w-0 truncate ${n === 0 && !pressed ? 'text-muted' : ''}`}>
        {label}
      </span>
      <span data-testid="n" className={`${SHELF_N} min-w-[3ch] flex-none text-end`}>
        {n}
      </span>
    </button>
  );
}
