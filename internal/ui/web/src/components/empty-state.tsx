import type { ReactNode } from 'react';

// EmptyState is what a list says in place of its rows: its words in the middle, the first of them bolder and larger,
// drawn hidden till it is the one shown, as the tests find each by its id
export function EmptyState({
  id,
  hidden,
  children,
}: {
  readonly id: string;
  readonly hidden: boolean;
  readonly children: ReactNode;
}) {
  return (
    <div
      id={id}
      data-testid="state"
      hidden={hidden}
      className="grid justify-items-center gap-2 px-6 py-14 text-center text-muted [&_p]:m-0 [&_p]:max-w-[34rem] [&_strong]:text-[1rem] [&_strong]:text-fg"
    >
      {children}
    </div>
  );
}
