import type { ReactNode } from 'react';

import { Badge } from '@/components/badge';

// PartHead heads a part of a task's page, its words and how many it holds on the left (none, no count)
// and the button that adds to it on the right: the parts
// of the family card (h3, under the task's h2), and its pages (h2, as the headings of their categories in #list are)
export function PartHead({
  level,
  id,
  hidden,
  place,
  words,
  count = 0,
  children,
}: {
  readonly level: 2 | 3;
  readonly id?: string;
  readonly hidden?: boolean;
  // place is the room about it, not how it looks
  readonly place: string;
  readonly words: ReactNode;
  readonly count?: number;
  readonly children: ReactNode;
}) {
  const H = level === 2 ? 'h2' : 'h3';
  return (
    <div id={id} hidden={hidden} className={`flex items-center justify-between gap-2 ${place}`}>
      <H className="m-0 flex items-center gap-[0.35rem] text-small leading-[1.4] font-bold text-fg [&_svg]:size-[15px] [&_svg]:flex-none [&_svg]:text-muted">
        {words}
        {/* The space is for what reads the heading whole, as the gap makes the room */}
        {count > 0 ? (
          <>
            {' '}
            <Badge>{count}</Badge>
          </>
        ) : null}
      </H>
      {children}
    </div>
  );
}
