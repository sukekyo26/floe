import {
  createContext,
  use,
  type FocusEventHandler,
  type KeyboardEventHandler,
  type MouseEventHandler,
  type ReactNode,
} from 'react';

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
  // wraps lets words a person typed go over lines, shown whole (ui.md: #tag-new, the one Button that wraps)
  readonly wraps?: boolean;
  // quiet: no ground and no line, a ground under the pointer (the bar of the selected); danger: red words (a delete);
  // armed: waiting for a second press, on red (data-armed). None changes its room: where it stands gives that
  readonly quiet?: boolean;
  readonly tone?: 'danger';
  readonly armed?: boolean;
  // adds: a button that opens what adds, its plus smaller
  readonly adds?: boolean;
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
  // onBlur lets go of a wait for a second press as the focus leaves (the bulk delete)
  readonly onBlur?: FocusEventHandler<HTMLButtonElement>;
}

// href makes it a link of the same shape (the way back of a page not found), which takes none of what a button does but
// a press (the bar's new pages: a link that, pressed plainly, narrows the list in place)
type ButtonLink = { readonly href: string; readonly onClick?: MouseEventHandler<HTMLAnchorElement> } & Readonly<
  Partial<Record<Exclude<keyof ButtonActs, 'onClick'>, never>>
>;

export type ButtonProps = ButtonShape & ((ButtonActs & { readonly href?: undefined }) | ButtonLink);

// Button is the one button of words (ui.md), its words on one line; where they do not fit, the row of buttons goes onto
// another
export function Button(props: ButtonProps) {
  const {
    children,
    kind = 'plain',
    adds = false,
    size: named,
    type = 'button',
    pressed,
    disabled,
    hidden,
    id,
    testid,
    label,
  } = props;
  const { closes, focusOnOpen, wraps, slug, onKeyDown, onBlur, quiet = false, tone, armed = false } = props;
  const size = named ?? use(ButtonSizeOf);
  const className = `inline-flex items-center justify-center gap-1.25 rounded-small border no-underline transition-[background-color,border-color,filter] duration-150 disabled:cursor-progress disabled:opacity-60 ${adds ? '[&_svg]:size-3.25' : '[&_svg]:size-3.75'} ${wraps ? 'min-h-8 px-2.5 py-1.25 text-start text-small wrap-anywhere' : size === 'foot' ? 'h-9 px-3.5 text-body whitespace-nowrap' : 'h-8 px-2.5 text-small whitespace-nowrap'} ${kind === 'primary' || pressed ? 'border-transparent bg-ink text-on-ink hover:bg-ink-hover' : quiet ? 'border-transparent bg-transparent hover:bg-subtle' : 'border-line bg-panel hover:border-faint'} ${kind === 'primary' || pressed ? '' : tone === 'danger' ? 'text-danger' : 'text-fg'} data-armed:bg-danger data-armed:text-on-danger data-armed:hover:bg-danger ${kind === 'primary' ? 'font-semibold' : ''}`;
  if (props.href !== undefined)
    return (
      <a
        id={id}
        data-testid={testid}
        aria-label={label}
        title={label}
        hidden={hidden}
        href={props.href}
        onClick={props.onClick}
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
      data-armed={armed ? 'true' : undefined}
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      disabled={disabled}
      hidden={hidden}
      onClick={props.onClick}
      onKeyDown={onKeyDown}
      onBlur={onBlur}
      className={className}
    >
      {children}
    </button>
  );
}
