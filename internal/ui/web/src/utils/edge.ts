// edgeStep is the width a key moves an edge to: 16 px either way from the width shown, Home and End its bounds; null
// for any other key
export function edgeStep(key: string, now: number, min: number, max: number): number | null {
  switch (key) {
    case 'ArrowLeft':
      return now - 16;
    case 'ArrowRight':
      return now + 16;
    case 'Home':
      return min;
    case 'End':
      return max;
    default:
      return null;
  }
}

// The narrowest a side column (the frame's tree, the diff's contents) is dragged to
export const SIDE_MIN = 160;

// sideMax is the widest a side column is dragged to in a window innerWidth wide: half of it, up to 480 px, never under
// SIDE_MIN
export const sideMax = (innerWidth: number): number => Math.max(SIDE_MIN, Math.min(480, Math.floor(innerWidth / 2)));

// clampSide is a side column's width kept within its bounds in a window innerWidth wide
export const clampSide = (width: number, innerWidth: number): number =>
  Math.min(Math.max(Math.round(width), SIDE_MIN), sideMax(innerWidth));

// shownSide is what a side column takes of the columns: its width within bounds, 0 folded
export const shownSide = (open: boolean, width: number, innerWidth: number): number =>
  open ? clampSide(width, innerWidth) : 0;
