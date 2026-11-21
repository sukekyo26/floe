import type { KeyboardEventHandler, MouseEventHandler, ReactNode } from 'react';

export interface ButtonProps {
  readonly children: ReactNode;
  readonly kind?: 'plain' | 'primary';
  // bar: where it stands among the screen's; foot: a modal's foot, taller
  readonly size?: 'bar' | 'foot';
  readonly type?: 'button' | 'submit';
  // A button that toggles: whether it is on (aria-pressed); left out, it toggles nothing
  readonly pressed?: boolean;
  readonly disabled?: boolean;
  readonly id?: string;
  readonly testid?: string | undefined;
  readonly label?: string | undefined;
  // closes marks a modal's close button (data-close), which the browser tests press
  readonly closes?: boolean;
  // slug names the page it acts on (data-slug), for it to be found again by its page
  readonly slug?: string | undefined;
  readonly onClick?: MouseEventHandler<HTMLButtonElement>;
  readonly onKeyDown?: KeyboardEventHandler<HTMLButtonElement>;
}

// Button is the one button of words (ui.md: .btn), its words on one line; where they do not fit, the row of buttons
// goes onto another
export function Button({
  children,
  kind = 'plain',
  size = 'bar',
  type = 'button',
  pressed,
  disabled,
  id,
  testid,
  label,
  closes,
  slug,
  onClick,
  onKeyDown,
}: ButtonProps) {
  return (
    <button
      // oxlint-disable-next-line react/button-has-type -- type is one of the two a button may be
      type={type}
      id={id}
      data-testid={testid}
      data-close={closes ? '' : undefined}
      data-slug={slug}
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      disabled={disabled}
      onClick={onClick}
      onKeyDown={onKeyDown}
      className={`inline-flex items-center justify-center gap-1.25 rounded-small border whitespace-nowrap transition-[background-color,border-color,filter] duration-150 disabled:cursor-progress disabled:opacity-60 [&_svg]:size-3.75 ${size === 'foot' ? 'h-9 px-3.5 text-body' : 'h-8 px-2.5 text-small'} ${kind === 'primary' || pressed ? 'border-transparent bg-ink text-on-ink hover:bg-ink-hover' : 'border-line bg-panel text-fg hover:border-faint'} ${kind === 'primary' ? 'font-semibold' : ''}`}
    >
      {children}
    </button>
  );
}
