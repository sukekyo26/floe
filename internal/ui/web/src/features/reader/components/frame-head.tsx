import { t } from '@/lib/t';
import type { Page } from '@/types/page';

interface FrameHeadProps {
  // page is the page found, null for none; file is a folder's file open, '' for none
  readonly page: Page | null;
  readonly file: string;
}

// FrameHead is the title of a page whole in its bar: a folder's file under its folder's title, and the archived mark.
// At 40rem or less it has no room to stand apart by a line
export function FrameHead({ page: a, file }: FrameHeadProps) {
  const name = (file || a?.title) ?? '';
  return (
    <>
      <span
        id="frame-title"
        title={a ? name : ''}
        className="relative ms-2.5 min-w-0 truncate ps-3.5 font-[650] before:absolute before:inset-y-1 before:left-0 before:border-l before:border-line before:content-[''] empty:hidden narrow:ms-0 narrow:ps-0 narrow:before:hidden"
      >
        {a && file ? <span className="crumb font-medium text-muted">{`${a.title ?? a.slug} / `}</span> : null}
        {a ? name : ''}
      </span>
      <span
        id="frame-archived"
        data-testid="badge"
        hidden={!a?.archived}
        className="flex-none rounded-full border border-line bg-subtle px-2 py-0.5 text-label font-semibold text-muted"
      >
        {t('ui_archived_badge')}
      </span>
    </>
  );
}
