import type { CSSProperties } from 'react';

import { TINT_DOT, TINT_VARS } from '@/components/tint';
import type { CssVars } from '@/utils/css-vars';

// PILL is the shape of a row's pills: a new version, the project, a file gone
export const PILL = 'rounded-full px-2 whitespace-nowrap';

export interface ProjectPillProps {
  readonly word: string;
  // color is the project's (projectColor)
  readonly color: string;
  readonly id?: string;
  // place is where it stands, and how wide it may be: a row's 10rem when not given
  readonly place?: string;
}

// ProjectPill is a project as a pill in its colour, its words cut at its end: on a row and on a task's head
export function ProjectPill({ word, color, id, place = 'max-w-40' }: ProjectPillProps) {
  const tint: CssVars = { '--c': color };
  return (
    <span
      id={id}
      data-testid="proj"
      // Painted in its project's colour, as it is drawn only then
      data-tint="true"
      style={tint as CSSProperties}
      className={`${PILL} inline-flex ${place} items-center gap-1.25 bg-(--tint) font-medium text-(--tint-ink) ${TINT_VARS} ${TINT_DOT}`}
    >
      <span data-testid="proj-name" className="min-w-0 truncate">
        {word}
      </span>
    </span>
  );
}
