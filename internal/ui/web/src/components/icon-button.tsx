import type { FocusEventHandler, MouseEventHandler, ReactNode, Ref } from 'react';

// Where it shows, by the bar's one rule of width (ui.md): shown; never (a menu's item a bar or a row shows, its CSS
// hiding it); narrow (a menu's item shown only where the bar folds it, under 40rem); wide (a bar's, folded into its menu
// under 40rem); screen (shown, but not printed: what opens a folded column, over the words)
export type Display = 'shown' | 'never' | 'narrow' | 'wide' | 'screen';

export interface IconButtonProps {
  // label is its name (aria-label), what a press does; title shows it to the pointer, the label when left out
  readonly label?: string | undefined;
  readonly title?: string | undefined;
  readonly children: ReactNode;
  // bar: 30 px, among the bar's; row: 26 px, a row's; chip: 28 px, a pin's ×; help: 24 px, the pins' help (#pins-help);
  // item: an item of a menu, as wide as the menu, its words beside
  readonly size?: 'bar' | 'row' | 'chip' | 'help' | 'kin' | 'item';
  readonly display?: Display;
  // A button that toggles: whether it is on (aria-pressed, "true" or "false"); left out, it toggles nothing
  readonly pressed?: boolean | undefined;
  // on: a button whose words say what pressing does (Restore, Close the diff), so not told as pressed
  readonly on?: boolean | undefined;
  // armed: a delete waiting for its second press
  readonly armed?: boolean | undefined;
  // raised: on a ground of its own with a shadow, over whatever lies under it (a column folded)
  readonly raised?: boolean | undefined;
  // expanded tells whether what it folds and opens shows (aria-expanded), controls names that (aria-controls)
  readonly expanded?: boolean | undefined;
  readonly controls?: string | undefined;
  // popoverTarget is the popover it shows and hides (the pins' help)
  readonly popoverTarget?: string | undefined;
  // tagged marks the tag line's button of a page with tags (data-tagged, P0b-2)
  readonly tagged?: boolean | undefined;
  readonly tone?: 'star' | 'danger' | undefined;
  readonly href?: string | undefined;
  readonly newTab?: boolean | undefined;
  readonly download?: boolean | undefined;
  readonly id?: string | undefined;
  readonly testid?: string | undefined;
  // task names the task it acts on (data-task): a row's of a task's family card
  readonly task?: string | undefined;
  readonly hidden?: boolean | undefined;
  readonly disabled?: boolean | undefined;
  // unpressable: reached by the keys and pressed, it says why it does nothing (aria-disabled: a category with pages)
  readonly unpressable?: boolean | undefined;
  readonly onClick?: MouseEventHandler<HTMLElement> | undefined;
  readonly onBlur?: FocusEventHandler<HTMLElement> | undefined;
  // ref holds a button the focus is given back to as it stays through redraws (the tag line's)
  readonly ref?: Ref<HTMLButtonElement> | undefined;
}

// A round one's padding, icon and size, by its size
const ROUND = {
  bar: { pad: 'px-1.5', svg: '[&_svg]:size-4', box: 'h-icon-btn min-w-icon-btn' },
  row: { pad: 'px-1.5', svg: '[&_svg]:size-4', box: 'h-6.5 min-w-6.5' },
  chip: { pad: 'px-0', svg: '[&_svg]:size-3', box: 'h-7 w-7 min-w-7 flex-none' },
  help: { pad: 'px-0', svg: '[&_svg]:size-4', box: 'h-6 w-6 min-w-6 flex-none' },
  kin: { pad: 'px-0', svg: '[&_svg]:size-4', box: 'h-7 min-w-7 flex-none' },
} as const;

const DISPLAY: Record<Display, string> = {
  shown: 'inline-flex',
  never: 'hidden',
  narrow: 'hidden narrow:inline-flex',
  wide: 'inline-flex narrow:hidden',
  screen: 'inline-flex print:hidden',
};

// IconButton is a button of an icon (and, in a menu, its words), round where it is not a menu's item
export function IconButton({
  label,
  title,
  children,
  size = 'bar',
  display = 'shown',
  pressed,
  on,
  armed,
  tagged,
  raised,
  expanded,
  controls,
  popoverTarget,
  tone,
  href,
  newTab,
  download,
  id,
  testid,
  task,
  hidden,
  disabled,
  unpressable,
  ref,
  onClick,
  onBlur,
}: IconButtonProps) {
  const className = `${DISPLAY[display]} items-center border border-transparent ${raised ? 'bg-panel shadow-sm ring-1 ring-line' : 'bg-transparent'} text-muted no-underline transition-colors duration-150 hover:bg-subtle disabled:cursor-default disabled:bg-transparent disabled:text-muted disabled:opacity-35 aria-disabled:cursor-default aria-disabled:bg-transparent aria-disabled:text-muted aria-disabled:opacity-35 aria-pressed:[&_svg]:fill-current data-armed:border-armed-line data-armed:bg-danger-soft data-armed:font-[650] data-armed:text-danger data-on:bg-ink data-on:text-on-ink data-on:hover:bg-ink-hover data-on:hover:text-on-ink ${size === 'item' ? 'min-h-9 w-full justify-start gap-2.5 rounded-[calc(var(--radius-menu)-var(--spacing-menu-pad))] px-2.5 py-1.75 text-start text-small leading-[1.35] [&_svg]:size-4 [&_svg]:shrink-0' : `justify-center gap-1.5 rounded-full ${ROUND[size].pad} data-on:rounded-[50%] ${ROUND[size].svg} ${ROUND[size].box}`} ${tone === 'star' ? 'hover:text-star aria-pressed:text-star' : tone === 'danger' ? 'hover:text-danger aria-pressed:text-fg' : 'hover:text-fg aria-pressed:text-fg'}`;
  const common = {
    id,
    'data-testid': testid,
    'data-task': task,
    'aria-label': label,
    title: title ?? (size === 'item' ? undefined : label),
    hidden,
    className,
    onBlur,
  };
  if (href !== undefined)
    return (
      <a
        {...common}
        href={href}
        target={newTab ? '_blank' : undefined}
        rel={newTab ? 'noreferrer' : undefined}
        download={download ? '' : undefined}
        data-on={on ? 'true' : undefined}
        onClick={onClick}
      >
        {children}
      </a>
    );
  return (
    <button
      ref={ref}
      {...common}
      type="button"
      aria-pressed={pressed}
      aria-expanded={expanded}
      aria-controls={controls}
      popoverTarget={popoverTarget}
      data-on={on ? 'true' : undefined}
      data-armed={armed ? 'true' : undefined}
      data-tagged={tagged ? 'true' : undefined}
      disabled={disabled}
      aria-disabled={unpressable ? 'true' : undefined}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
