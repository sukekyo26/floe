import { t } from '@/lib/t';
import type { Card } from '@/features/settings/utils/settings-view';

interface ThemeCardProps {
  readonly card: Card;
  readonly checked: boolean;
  readonly onPick: () => void;
}

const PILL = 'rounded-full px-1.75 py-px text-label font-semibold';

// ThemeCard is a theme to choose: its sample, drawn by the server in the sandbox and never used, shrunk to the card
// (useShrink); the card that takes a page's theme off has its about in the sample's place
export function ThemeCard({ card, checked, onPick }: ThemeCardProps) {
  return (
    <label
      data-testid="theme-card"
      className="relative grid cursor-pointer content-start gap-1.5 rounded-menu border border-line bg-frame-bg px-2.25 pt-2.25 pb-2.75 hover:bg-hover has-checked:outline-2 has-checked:outline-offset-1 has-checked:outline-ink has-checked:after:absolute has-checked:after:top-3.5 has-checked:after:right-3.5 has-checked:after:size-6.5 has-checked:after:rounded-full has-checked:after:bg-ink has-checked:after:[background-image:var(--tick)] has-checked:after:bg-size-[16px] has-checked:after:bg-center has-checked:after:bg-no-repeat has-checked:after:content-[''] has-[input:focus-visible:not(:checked)]:[outline:var(--ring)] has-[input:focus-visible:not(:checked)]:outline-offset-2"
    >
      <input
        type="radio"
        name="theme"
        value={card.name}
        checked={checked}
        onChange={onPick}
        className="absolute opacity-0"
      />
      {card.name === '' ? (
        <span
          data-testid="theme-shot"
          className="relative grid aspect-[900/640] place-items-center overflow-hidden rounded-small border border-line bg-subtle p-4 text-center text-small leading-[1.6] text-muted narrow:p-1.5 narrow:text-label narrow:leading-[1.4]"
        >
          {card.about}
        </span>
      ) : (
        <span
          data-testid="theme-shot"
          className="relative aspect-[900/640] overflow-hidden rounded-small border border-line bg-subtle"
        >
          <iframe
            src={`/api/themes/${encodeURIComponent(card.name)}/sample`}
            loading="lazy"
            tabIndex={-1}
            title={card.name}
            sandbox=""
            aria-hidden="true"
            className="pointer-events-none absolute top-0 left-0 h-[640px] w-[900px] origin-top-left scale-(--shrink,0.25) border-0"
          />
        </span>
      )}
      <span data-testid="nm" className="flex flex-wrap items-center gap-1.5 text-body font-semibold">
        {card.name === '' ? t('ui_theme_unset') : card.name}
        {card.now ? <span className={`${PILL} bg-ink text-on-ink`}>{t('ui_theme_now')}</span> : null}
        {card.dir ? <span className={`${PILL} bg-sel text-muted`}>{t('ui_theme_own')}</span> : null}
      </span>
      {card.name !== '' && card.about ? (
        <span data-testid="ds" className="text-label leading-[1.45] text-muted max-[600px]:hidden">
          {card.about}
        </span>
      ) : null}
      {card.dir ? <span className="font-mono text-label wrap-anywhere text-faint">{card.dir}</span> : null}
    </label>
  );
}
