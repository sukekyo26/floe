import { useState } from 'react';

import { edgeStep } from '@/utils/edge';

export interface GripProps {
  readonly id: string;
  readonly label: string;
  // controls is the id of what it widens
  readonly controls: string;
  // at is where it stands from the window's left; width the width it sets now, within min and max
  readonly at: number;
  readonly width: number;
  readonly min: number;
  readonly max: number;
  // left is where the width starts, read as it is dragged
  readonly left?: () => number;
  // onSet takes the width asked for; onKeep keeps it, once the drag or the key stops
  readonly onSet: (width: number) => void;
  readonly onKeep: () => void;
  readonly hidden?: boolean;
}

// Grip is an edge to drag, or to move by keys (ui.md: dragEdge), between columns: a line shows while it is dragged,
// pointed at or focused. Its value and bounds are for its keys and for screen readers
export function Grip({ id, label, controls, at, width, min, max, left = () => 0, onSet, onKeep, hidden }: GripProps) {
  const [dragging, setDragging] = useState(false);
  return (
    <div
      id={id}
      // A splitter one moves (the window splitter pattern), which hr, a separator of nothing, is not
      // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- hr takes no focus, value or keys
      role="separator"
      aria-orientation="vertical"
      aria-controls={controls}
      aria-label={label}
      aria-valuenow={width}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      hidden={hidden}
      data-dragging={dragging ? 'true' : undefined}
      style={{ left: `${at - 3}px` }}
      onPointerDown={(e) => {
        if (e.button) return;
        e.preventDefault();
        e.currentTarget.setPointerCapture(e.pointerId);
        setDragging(true);
      }}
      onPointerMove={(e) => {
        if (e.currentTarget.hasPointerCapture(e.pointerId)) onSet(e.clientX - left());
      }}
      onLostPointerCapture={() => {
        setDragging(false);
        onKeep();
      }}
      onKeyDown={(e) => {
        const to = edgeStep(e.key, width, min, max);
        if (to === null) return;
        e.preventDefault();
        onSet(to);
        onKeep();
      }}
      className="fixed top-bar bottom-0 z-6 w-1.5 cursor-col-resize touch-none after:absolute after:inset-y-0 after:left-0.5 after:w-0.5 after:bg-transparent after:transition-colors after:duration-150 after:content-[''] hover:after:bg-ink focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-faint focus-visible:after:bg-ink data-dragging:after:bg-ink"
    />
  );
}
