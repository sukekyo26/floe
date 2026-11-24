import type { ReactNode } from 'react';

// ListHeadBox is the head over the rows of a list, of the page list and the task list: what narrows it and how many,
// or, while pages are selected, the bulk bar alone on the ground of the rows selected
export function ListHeadBox({ hidden = false, children }: { readonly hidden?: boolean; readonly children: ReactNode }) {
  return (
    <div
      data-testid="lhead"
      hidden={hidden}
      className="@container/bar sticky top-0 z-5 flex h-lhead items-center gap-2.5 border-b border-line bg-panel px-4 text-small text-muted has-[>#bulk:not([hidden])]:bg-sel has-[>#bulk:not([hidden])]:px-2 has-[>#bulk:not([hidden])]:[&>:not(#bulk)]:hidden"
    >
      {children}
    </div>
  );
}
