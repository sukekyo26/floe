import type { ReactNode } from 'react';

export interface ChipGroupProps {
  // id is the part's (shelf-projects), boxId its chips' (chips); title its heading and the name of its group
  readonly id: string;
  readonly boxId: string;
  readonly title: string;
  // hidden is the caller's: the page list hides a part with none to choose, the task list keeps its projects
  readonly hidden: boolean;
  readonly children: ReactNode;
}

// ChipGroup is a part of the shelf under its heading: the projects, the kinds, the templates or the tags
export function ChipGroup({ id, boxId, title, hidden, children }: ChipGroupProps) {
  return (
    <div id={id} hidden={hidden}>
      <h2 className="m-0 mb-[0.3rem] ml-[0.65rem] text-label leading-[1.2] font-semibold tracking-[0.08em] text-muted">
        {title}
      </h2>
      {/* oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- a fieldset would draw a box and a legend */}
      <div id={boxId} role="group" aria-label={title} className="grid grid-cols-[minmax(0,1fr)] gap-1">
        {children}
      </div>
    </div>
  );
}
