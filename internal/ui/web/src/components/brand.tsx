import { t } from '@/lib/t';
import { barPress } from '@/stores/columns';

// Brand is the bar's logo and name, the way to the page list: to href, the list as this tab last had it in a page
// whole. It is its mark alone at 24rem or less, where the links beside it take the word's room, and a page whole's at
// 40rem or less; its word is still the link's own text, and so its name
export function Brand({
  href = '/',
  markAlone = false,
}: {
  readonly href?: string | undefined;
  readonly markAlone?: boolean;
}) {
  return (
    <a
      data-testid="brand"
      href={href}
      title={t('ui_to_page_list')}
      onClick={barPress()}
      className={`flex flex-none items-center gap-2.5 text-[1.2rem] leading-none font-extrabold tracking-[-0.03em] text-fg no-underline tiny:gap-0 tiny:text-[0px] ${markAlone ? 'narrow:gap-0 narrow:text-[0px]' : ''}`}
    >
      <svg
        viewBox="0 0 64 64"
        aria-hidden="true"
        className="size-6.5 flex-none drop-shadow-[0_2px_6px_var(--color-brand-glow)]"
      >
        <defs>
          <linearGradient id="floe-sky" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" className="[stop-color:var(--color-logo-sky-top)]" />
            <stop offset="1" className="[stop-color:var(--color-logo-sky-bottom)]" />
          </linearGradient>
        </defs>
        <rect width="64" height="64" rx="15" fill="url(#floe-sky)" />
        <path d="M17 47.5H47" fill="none" className="stroke-logo-line" strokeWidth="2" strokeLinecap="round" />
        <path d="M24 52.5H40" fill="none" className="stroke-logo-line-soft" strokeWidth="2" strokeLinecap="round" />
        <path d="M12 32 22 38 41 37 53 29.5 53 34.5 41 42 22 43 12 37Z" className="fill-logo-edge" />
        <path d="M12 32 24 21.5 33 28 22 38Z" className="fill-logo-face-left" />
        <path d="M24 21.5 42 21 53 29.5 33 28Z" className="fill-logo-face-back" />
        <path d="M33 28 53 29.5 41 37 22 38Z" className="fill-logo-face-front" />
      </svg>
      floe
    </a>
  );
}
