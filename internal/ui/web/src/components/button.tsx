import { createContext, use, type KeyboardEventHandler, type MouseEventHandler, type ReactNode } from 'react';

type ButtonSize = 'bar' | 'foot';

// ButtonSizeOf is the size the buttons inside take when they name none: a modal's foot gives its own (DialogActions)
export const ButtonSizeOf = createContext<ButtonSize>('bar');

interface ButtonShape {
  readonly children: ReactNode;
  readonly kind?: 'plain' | 'primary';
  // bar: where it stands among the screen's; foot: a modal's foot, taller. Left out, the size of where it is (ButtonSizeOf)
  readonly size?: ButtonSize;
  readonly hidden?: boolean;
  readonly id?: string;
  readonly testid?: string | undefined;
  readonly label?: string | undefined;
  // wraps lets words a person typed go over lines, shown whole (ui.md: #tag-new, the one .btn that wraps)
  readonly wraps?: boolean;
}

interface ButtonActs {
  readonly type?: 'button' | 'submit';
  // A button that toggles: whether it is on (aria-pressed); left out, it toggles nothing
  readonly pressed?: boolean;
  readonly disabled?: boolean;
  // closes marks a modal's close button (data-close), which the browser tests press
  readonly closes?: boolean;
  // focusOnOpen marks the button its modal focuses as it opens (data-autofocus: Dialog)
  readonly focusOnOpen?: boolean;
  // slug names the page it acts on (data-slug), for it to be found again by its page
  readonly slug?: string | undefined;
  readonly onClick?: MouseEventHandler<HTMLButtonElement>;
  readonly onKeyDown?: KeyboardEventHandler<HTMLButtonElement>;
}

// href makes it a link of the same shape (ui.html a.btn: the way back of a page not found), which takes none of what
// a button does
type ButtonLink = { readonly href: string } & { readonly [K in keyof ButtonActs]?: never };

export type ButtonProps = ButtonShape & ((ButtonActs & { readonly href?: undefined }) | ButtonLink);

// Button is the one button of words (ui.md: .btn), its words on one line; where they do not fit, the row of buttons
// goes onto another
export function Button({
  children,
  kind = 'plain',
  size: named,
  type = 'button',
  pressed,
  disabled,
  hidden,
  id,
  testid,
  label,
  closes,
  focusOnOpen,
  wraps,
  slug,
  href,
  onClick,
  onKeyDown,
}: ButtonProps) {
  const size = named ?? use(ButtonSizeOf);
  const className = `inline-flex items-center justify-center gap-1.25 rounded-small border no-underline transition-[background-color,border-color,filter] duration-150 disabled:cursor-progress disabled:opacity-60 [&_svg]:size-3.75 ${wraps ? 'min-h-8 px-2.5 py-1.25 text-start text-small wrap-anywhere' : size === 'foot' ? 'h-9 px-3.5 text-body whitespace-nowrap' : 'h-8 px-2.5 text-small whitespace-nowrap'} ${kind === 'primary' || pressed ? 'border-transparent bg-ink text-on-ink hover:bg-ink-hover' : 'border-line bg-panel text-fg hover:border-faint'} ${kind === 'primary' ? 'font-semibold' : ''}`;
  if (href !== undefined)
    return (
      <a
        id={id}
        data-testid={testid}
        aria-label={label}
        title={label}
        hidden={hidden}
        href={href}
        className={className}
      >
        {children}
      </a>
    );
  return (
    <button
      // oxlint-disable-next-line react/button-has-type -- type is one of the two a button may be
      type={type}
      id={id}
      data-testid={testid}
      data-close={closes ? '' : undefined}
      data-autofocus={focusOnOpen ? '' : undefined}
      data-slug={slug}
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      disabled={disabled}
      hidden={hidden}
      onClick={onClick}
      onKeyDown={onKeyDown}
      className={className}
    >
      {children}
    </button>
  );
}
