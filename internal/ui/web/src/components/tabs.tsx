import type { KeyboardEvent, ReactNode } from 'react';

import { nextTab } from '@/utils/tabs';

// TabList is a row of tabs over the panels they show, a thin line under it that the chosen tab's line sits on. ← and
// → go from tab to tab and choose it; Tab goes on to the panel shown (its caller gives it tabIndex 0), the tabs not
// chosen being out of the Tab order
export function TabList({ label, children }: { readonly label: string; readonly children: ReactNode }) {
  return (
    <div role="tablist" aria-label={label} className="flex flex-wrap gap-x-5 border-b border-line">
      {children}
    </div>
  );
}

// tabLook is a tab's look: underlined and darker when it is the one now, its weight the same so that a switch moves
// none
const tabLook = (now: boolean): string =>
  `border-0 border-b-2 text-small whitespace-nowrap transition-colors duration-150 ${now ? 'border-ink text-fg' : 'border-transparent text-muted hover:text-fg'}`;

// moveTab is ← or → on a tab: the next tab of its list takes the focus and is chosen
function moveTab(e: KeyboardEvent<HTMLButtonElement>) {
  const tabs = [...(e.currentTarget.parentElement?.querySelectorAll<HTMLElement>('[role=tab]:not([hidden])') ?? [])];
  const at = tabs.indexOf(e.currentTarget);
  const to = nextTab(at, tabs.length, e.key);
  if (to === at) return;
  e.preventDefault();
  tabs[to]?.focus();
  tabs[to]?.click();
}

interface TabProps {
  // id names the tab, which its panel is labelled by
  readonly id: string;
  readonly selected: boolean;
  readonly onSelect: () => void;
  // help names the help's tab (data-help), as the tests read it
  readonly help?: string;
  readonly children: ReactNode;
}

// Tab is one tab: its words alone, the chosen one underlined over the list's line and darker, its weight the same so
// that a switch moves no tab (the help's test measures them). Its panel is told by the panel's aria-labelledby alone:
// the help's tabs have no aria-controls
export function Tab({ id, selected, onSelect, help, children }: TabProps) {
  return (
    <button
      type="button"
      role="tab"
      id={id}
      aria-selected={selected}
      tabIndex={selected ? 0 : -1}
      data-help={help}
      onClick={onSelect}
      onKeyDown={moveTab}
      className={`-mb-px bg-transparent px-0.5 pt-1 pb-2 ${tabLook(selected)}`}
    >
      {children}
    </button>
  );
}
