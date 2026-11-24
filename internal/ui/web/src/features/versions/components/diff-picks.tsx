import { IconButton } from '@/components/icon-button';
import { OpenIcon } from '@/components/icons';
import { PickSelect } from '@/components/pick-select';
import { pairOptions, type Pair } from '@/features/versions/utils/diff-pair';
import type { VersionList } from '@/features/versions/utils/version-words';
import { t } from '@/lib/t';
import { rawHref } from '@/utils/page-urls';

interface DiffPicksProps {
  readonly slug: string;
  readonly list: VersionList | null;
  readonly pair: Pair | null;
  readonly onChoose: (name: keyof Pair, n: number) => void;
}

const WORDS = { from: 'ui_diff_before', to: 'ui_diff_after' } as const;

// DiffPicks are the two versions compared, newest first, one that would not be older or later than the other not to be
// chosen, each with a way to its page alone in a new tab
export function DiffPicks({ slug, list, pair, onChoose }: DiffPicksProps) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 narrow:w-full">
      {(['from', 'to'] as const).map((name) => {
        const n = pair?.[name];
        const open = n === undefined ? '' : t('ui_diff_open_version', n);
        return (
          <div key={name} className="flex min-w-0 items-center gap-1.5 text-muted narrow:w-full">
            <label htmlFor={`diff-${name}`} className="whitespace-nowrap narrow:min-w-[3.2rem] narrow:flex-none">
              {t(WORDS[name])}
            </label>
            <PickSelect
              id={`diff-${name}`}
              label={t(WORDS[name])}
              value={n === undefined ? '' : String(n)}
              onChange={(e) => {
                onChoose(name, Number(e.currentTarget.value));
              }}
            >
              {list && pair
                ? pairOptions(name, pair, list).map((o) => (
                    <option key={o.n} value={o.n} disabled={o.off}>
                      {o.text}
                    </option>
                  ))
                : null}
            </PickSelect>
            {/* None until the pair is known: a way to no page, which no one could take */}
            {n === undefined ? null : (
              <IconButton id={`diff-${name}-open`} label={open} href={rawHref({ slug, v: n })} newTab>
                <OpenIcon />
              </IconButton>
            )}
          </div>
        );
      })}
    </div>
  );
}
