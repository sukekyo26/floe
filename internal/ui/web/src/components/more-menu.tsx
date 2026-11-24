import type { MouseEventHandler, ReactNode } from 'react';
import { flushSync } from 'react-dom';

import { IconButton, type IconButtonProps } from '@/components/icon-button';
import { MoreIcon } from '@/components/icons';
import { useStore } from '@/hooks/use-store';
import { menus } from '@/stores/menus';

export interface MoreMenuProps {
  // id names the menu among those of the screen, one open at a time (stores/menus.ts)
  readonly id: string;
  // label names its button: what it is the menu of
  readonly label: string;
  // row: a row's, its button 26 px; bar: the bar's over a page, 30 px
  readonly size?: 'row' | 'bar';
  // opens: down under its button; folds, the frame's bar, whose menu under 40rem stands under the bar, in from the
  // window's right, as under its own button it would run off the left of a narrow window; up, the search's foot, whose
  // menu stands over the foot from its left, over the rows
  // LIMIT: up, on a screen under some 320px high its first items are off it, with nothing to scroll
  readonly opens?: 'down' | 'folds' | 'up';
  readonly children: ReactNode;
}

const PLACE = { down: 'relative', folds: 'relative narrow:static', up: 'static' } as const;
const MENU = {
  down: 'top-[calc(100%+4px)] right-0 max-w-[calc(100vw-2rem)] min-w-70',
  folds:
    'top-[calc(100%+4px)] right-0 max-w-[calc(100vw-2rem)] min-w-70 narrow:right-4 narrow:min-w-[min(17.5rem,calc(100vw-2rem))]',
  up: 'bottom-[calc(100%-0.5rem)] left-0 max-w-full min-w-[min(17.5rem,100%)]',
} as const;

// MoreMenu is the menu "…" of a row or of the bar over a page: a details whose open is the one open of stores/menus.ts,
// its items made as it opens: those of hundreds of rows would hold the list's first draw back. What closes it (a click
// elsewhere, an item done, Escape, the window left) is useOpenMenus, one for the screen
export function MoreMenu({ id, label, size = 'row', opens = 'down', children }: MoreMenuProps) {
  const open = useStore(menus.open) === id;
  return (
    <details
      data-menu-id={id}
      data-testid="more"
      open={open}
      // Opened or shut by other than its press (the browser, a script), the store follows: the items are the store's
      onToggle={(e) => {
        if (e.currentTarget.open) menus.show(id);
        else menus.hide(id);
      }}
      className={`flex [&>summary]:list-none [&>summary::-webkit-details-marker]:hidden ${PLACE[opens]}`}
    >
      <summary
        // The store first, the details drawn from it at once, as the browser's own press would: its toggle event comes a
        // task later, and an item pressed before it would find no menu open to close
        onClick={(e) => {
          e.preventDefault();
          flushSync(() => {
            if (menus.open.get() === id) menus.hide(id);
            else menus.show(id);
          });
        }}
        aria-label={label}
        title={label}
        className={`inline-flex cursor-pointer items-center justify-center rounded-full border border-transparent px-1.5 text-muted transition-colors duration-150 hover:bg-subtle hover:text-fg [&_svg]:size-4 ${size === 'row' ? 'h-6.5 min-w-6.5' : 'h-icon-btn min-w-icon-btn'}`}
      >
        <MoreIcon />
      </summary>
      <div
        data-testid="menu"
        className={`absolute z-30 grid gap-0.5 rounded-menu border border-line bg-panel p-menu-pad shadow-md ${MENU[opens]}`}
      >
        {open ? children : null}
      </div>
    </details>
  );
}

export interface MenuItemProps extends Omit<IconButtonProps, 'size' | 'children' | 'onClick'> {
  readonly icon: ReactNode;
  readonly children: ReactNode;
  // keepsOpen leaves the menu open as it is pressed (a delete, whose first press asks for a second)
  readonly keepsOpen?: boolean;
  readonly onSelect?: MouseEventHandler<HTMLElement> | undefined;
}

// MenuItem is an item of a MoreMenu, its words in the label the tests read: done, it closes the menu unless it
// keepsOpen
export function MenuItem({ icon, children, keepsOpen, onSelect, ...rest }: MenuItemProps) {
  return (
    <span data-menu-item="" data-keeps-open={keepsOpen ? '' : undefined} className="contents">
      <IconButton {...rest} size="item" onClick={onSelect}>
        {icon}
        <span data-testid="label">{children}</span>
      </IconButton>
    </span>
  );
}
