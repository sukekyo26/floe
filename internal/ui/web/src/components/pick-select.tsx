import type { ChangeEventHandler, ReactNode, Ref } from 'react';

interface PickSelectProps {
  readonly id?: string | undefined;
  readonly label: string;
  readonly value: string;
  readonly onChange: ChangeEventHandler<HTMLSelectElement>;
  readonly children: ReactNode;
  // compact: the version's number alone closed, 1.75rem high, never shrunk (a page's version, a task's state); else 2rem
  // high, the whole width under 40rem (the diff's pair)
  readonly compact?: boolean;
  // old: a version other than the latest, in the colours of a line changed in part (data-old)
  readonly tone?: 'old' | undefined;
  // state is a task's state drawn (data-state), in its colours
  readonly state?: string | undefined;
  readonly hidden?: boolean | undefined;
  // numbers are the values listed, joined by commas, as the browser tests read them (data-numbers)
  readonly numbers?: string | undefined;
  readonly ref?: Ref<HTMLSelectElement> | undefined;
}

// A task's state's colours, by the states store.States names
const STATE_TONE: Readonly<Record<string, string>> = {
  doing: 'border-chg bg-chg-soft text-chg',
  todo: 'border-line bg-subtle text-muted',
  done: 'border-ins bg-ins-soft text-ins',
};

// PickSelect is a choice in a pill whose list opens as a menu: its button holds the chosen option's copy
// (selectedcontent). data-version-pick lets the screen's keys go on from it (use-keys). Its ring is the page's
// :focus-visible
// LIMIT: a browser without base-select (Firefox and Safari for now) shows its own list, in the rules outside supports-, which works the same
export function PickSelect({
  id,
  label,
  value,
  onChange,
  children,
  compact = false,
  tone,
  state,
  hidden,
  numbers,
  ref,
}: PickSelectProps) {
  const old = tone === 'old';
  return (
    <select
      ref={ref}
      id={id}
      aria-label={label}
      value={value}
      onChange={onChange}
      hidden={hidden}
      data-version-pick=""
      data-old={old ? 'true' : undefined}
      data-state={state}
      data-numbers={numbers}
      className={`h-8 max-w-full min-w-0 rounded-small border px-1.5 font-[inherit] supports-[appearance:base-select]:inline-flex supports-[appearance:base-select]:cursor-pointer supports-[appearance:base-select]:[appearance:base-select] supports-[appearance:base-select]:items-center supports-[appearance:base-select]:justify-between supports-[appearance:base-select]:rounded-full supports-[appearance:base-select]:text-small supports-[appearance:base-select]:leading-[1.4] supports-[appearance:base-select]:shadow-sm supports-[appearance:base-select]:open:outline-none supports-[appearance:base-select]:[&_option]:min-h-0 supports-[appearance:base-select]:[&_option]:cursor-pointer supports-[appearance:base-select]:[&_option]:rounded-[calc(var(--radius-menu)-var(--spacing-menu-pad))] supports-[appearance:base-select]:[&_option]:px-2.5 supports-[appearance:base-select]:[&_option]:py-1.5 supports-[appearance:base-select]:[&_option]:text-small supports-[appearance:base-select]:[&_option]:leading-[1.4] supports-[appearance:base-select]:[&_option]:font-normal supports-[appearance:base-select]:[&_option]:whitespace-nowrap supports-[appearance:base-select]:[&_option:checked]:bg-transparent supports-[appearance:base-select]:[&_option:checked]:font-bold supports-[appearance:base-select]:[&_option:checked:hover]:bg-subtle supports-[appearance:base-select]:[&_option:disabled]:cursor-default supports-[appearance:base-select]:[&_option:disabled]:bg-transparent supports-[appearance:base-select]:[&_option:disabled]:text-faint supports-[appearance:base-select]:[&_option:focus-visible]:bg-subtle supports-[appearance:base-select]:[&_option:focus-visible]:shadow-[inset_2px_0_0_var(--color-ink)] supports-[appearance:base-select]:[&_option:focus-visible]:outline-none supports-[appearance:base-select]:[&_option:hover]:bg-subtle supports-[appearance:base-select]:[&::picker(select)]:my-1 supports-[appearance:base-select]:[&::picker(select)]:max-h-56 supports-[appearance:base-select]:[&::picker(select)]:[appearance:base-select] supports-[appearance:base-select]:[&::picker(select)]:overflow-y-auto supports-[appearance:base-select]:[&::picker(select)]:rounded-menu supports-[appearance:base-select]:[&::picker(select)]:border supports-[appearance:base-select]:[&::picker(select)]:border-line supports-[appearance:base-select]:[&::picker(select)]:bg-panel supports-[appearance:base-select]:[&::picker(select)]:p-menu-pad supports-[appearance:base-select]:[&::picker(select)]:text-fg supports-[appearance:base-select]:[&::picker(select)]:shadow-md ${
        compact
          ? 'flex-none supports-[appearance:base-select]:h-7 supports-[appearance:base-select]:gap-1 supports-[appearance:base-select]:ps-2.5 supports-[appearance:base-select]:pe-1.25'
          : 'supports-[appearance:base-select]:gap-1.5 supports-[appearance:base-select]:ps-3 supports-[appearance:base-select]:pe-2 narrow:w-full'
      } ${
        old
          ? 'border-chg bg-chg-soft font-bold text-chg [&::picker-icon]:text-chg'
          : state !== undefined
            ? `font-bold [&::picker-icon]:text-current ${STATE_TONE[state] ?? 'border-line bg-panel text-fg'}`
            : 'border-box-line bg-panel text-fg supports-[appearance:base-select]:border-line supports-[appearance:base-select]:font-medium supports-[appearance:base-select]:open:border-faint supports-[appearance:base-select]:hover:border-faint supports-[appearance:base-select]:[&::picker-icon]:text-muted'
      }`}
    >
      {children}
    </select>
  );
}
