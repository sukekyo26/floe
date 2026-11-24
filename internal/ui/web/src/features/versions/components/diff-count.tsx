import { countParts, type Counts } from '@/features/versions/utils/diff-words';
import { t } from '@/lib/t';

const TONE: Record<string, string> = { c: 'text-chg', a: 'text-ins', r: 'text-del', s: 'text-fg' };

// DiffCount is what the answer counts, in words: a span each, or one sentence when all is the same
export function DiffCount({ counts }: { readonly counts: Counts | null }) {
  const parts = counts && countParts(counts);
  return (
    <output id="diff-count" className="m-0 flex flex-wrap gap-x-3 text-muted">
      {counts && !parts ? t('ui_diff_nothing') : null}
      {parts?.map((x) => (
        <span key={x.cls}>
          <b className={`font-bold ${TONE[x.cls] ?? ''}`}>{x.n}</b>
          {` ${t(x.word)}`}
        </span>
      ))}
    </output>
  );
}
