import type { ComponentPropsWithoutRef } from 'react';

// LINK_LOOK is the underline of a link among words, which Tailwind's preflight takes off every link: the one place it
// is written, for a button that reads as such a link too
export const LINK_LOOK = 'underline underline-offset-3';

const TONE = {
  fg: 'text-fg',
  // The words it stands among are a note's, so it is bolder as well as coloured
  new: 'font-bold text-new',
  chg: 'font-bold text-chg',
  inherit: 'text-inherit',
} as const;

export type TextLinkProps = Omit<ComponentPropsWithoutRef<'a'>, 'className'> & {
  // tone is its colour: the words' own (fg), a note's (new, chg), or that of what it is in (inherit)
  readonly tone?: keyof typeof TONE;
  // className is for where it is placed, not for how it looks
  readonly className?: string;
};

// TextLink is a link among words or in place of them: underlined, as nothing else tells it from the words. A link of
// another shape (a row's title, a button, the bar's) is not one.
export function TextLink({ tone = 'fg', className, children, ...rest }: TextLinkProps) {
  return (
    <a {...rest} className={[LINK_LOOK, TONE[tone], className].filter(Boolean).join(' ')}>
      {children}
    </a>
  );
}
