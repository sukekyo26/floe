import type { ReactNode } from 'react';

// Badge is a word or a count in a pill: on a row's line under its title, archived or where a page is on a task; in a
// heading, how many it heads
export function Badge({ children }: { readonly children: ReactNode }) {
  return (
    <span data-testid="badge" className="rounded-full border border-line bg-subtle px-[0.3rem] text-label text-muted">
      {children}
    </span>
  );
}
