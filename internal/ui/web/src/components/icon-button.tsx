import type { MouseEventHandler, ReactNode } from 'react';

export interface IconButtonProps {
  // label is its name (aria-label), what a press does; title shows it to the pointer, the label when left out
  readonly label?: string;
  readonly title?: string;
  readonly children: ReactNode;
  // bar: 30 px, among the bar's; row: 26 px, a row's; item: an item of a menu, as wide as the menu, its words beside
  readonly size?: 'bar' | 'row' | 'item';
  // A button that toggles: whether it is on (aria-pressed, "true" or "false"); left out, it toggles nothing
  readonly pressed?: boolean;
  // on: an archive button whose words say what pressing does (Restore), so not told as pressed (ui.html 1982)
  readonly on?: boolean;
  readonly tone?: 'star' | 'danger';
  readonly href?: string;
  readonly newTab?: boolean;
  readonly download?: boolean;
  readonly id?: string;
  readonly testid?: string;
  readonly hidden?: boolean;
  readonly disabled?: boolean;
  readonly onClick?: MouseEventHandler<HTMLElement>;
}

// IconButton is a button of an icon (and, in a menu, its words), round where it is not a menu's item
export function IconButton({
  label,
  title,
  children,
  size = 'bar',
  pressed,
  on,
  tone,
  href,
  newTab,
  download,
  id,
  testid,
  hidden,
  disabled,
  onClick,
}: IconButtonProps) {
  const className = `inline-flex items-center gap-1.5 border border-transparent bg-transparent text-muted no-underline transition-colors duration-150 hover:bg-subtle disabled:cursor-default disabled:bg-transparent disabled:text-muted disabled:opacity-35 aria-pressed:[&_svg]:fill-current data-on:bg-ink data-on:text-on-ink data-on:hover:bg-ink-hover data-on:hover:text-on-ink ${size === 'item' ? 'min-h-9 w-full justify-start gap-2.5 rounded-[calc(var(--radius-menu)-var(--spacing-menu-pad))] px-2.5 py-1.75 text-start text-small leading-[1.35] [&_svg]:size-4 [&_svg]:shrink-0' : `justify-center rounded-full px-1.5 data-on:rounded-[50%] [&_svg]:size-4 ${size === 'row' ? 'h-6.5 min-w-6.5' : 'h-icon-btn min-w-icon-btn'}`} ${tone === 'star' ? 'hover:text-star aria-pressed:text-star' : tone === 'danger' ? 'hover:text-danger aria-pressed:text-fg' : 'hover:text-fg aria-pressed:text-fg'}`;
  const common = {
    id,
    'data-testid': testid,
    'aria-label': label,
    title: title ?? (size === 'item' ? undefined : label),
    hidden,
    className,
  };
  if (href !== undefined)
    return (
      <a
        {...common}
        href={href}
        target={newTab ? '_blank' : undefined}
        rel={newTab ? 'noreferrer' : undefined}
        download={download ? '' : undefined}
        onClick={onClick}
      >
        {children}
      </a>
    );
  return (
    <button
      {...common}
      type="button"
      aria-pressed={pressed}
      data-on={on ? '' : undefined}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
