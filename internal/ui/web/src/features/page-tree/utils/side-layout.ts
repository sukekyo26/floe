import { clampSide } from '@/utils/edge';

export interface SideLaid {
  // shown: the tree shows; width: its width within the bounds of now; take: what it takes of the page's width, none
  // over the page
  shown: boolean;
  width: number;
  take: number;
}

// sideLayout lays the frame's tree out: beside the page as kept, or, in a narrow window, over the page as asked there
// (over), its width within the bounds of the window (what is kept is not changed by them)
export function sideLayout(at: {
  readonly open: boolean;
  readonly over: boolean;
  readonly narrow: boolean;
  readonly width: number;
  readonly innerWidth: number;
}): SideLaid {
  const shown = at.narrow ? at.over : at.open;
  const width = clampSide(at.width, at.innerWidth);
  return { shown, width, take: shown && !at.narrow ? width : 0 };
}
