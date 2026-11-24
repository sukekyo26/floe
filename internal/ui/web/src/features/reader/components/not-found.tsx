import { Button } from '@/components/button';
import { BackChevronIcon } from '@/components/icons';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';

interface NotFoundProps {
  // page is the entry of the slug asked, if listed (a link whose file is gone); slug is the one asked
  readonly page: Page | null;
  readonly slug: string;
  // back is the list to go back to (the list saved)
  readonly back: string;
  readonly hidden: boolean;
}

// NotFound is a page whole not there: a link's file gone, or no page of the slug
export function NotFound({ page: a, slug, back, hidden }: NotFoundProps) {
  return (
    <div
      id="notfound"
      data-testid="state"
      hidden={hidden}
      className="grid flex-1 content-center justify-items-center gap-2 px-6 py-14 text-center text-muted [&>svg]:size-13"
    >
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M7 3h7l5 5v12a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Zm7 0v5h5" />
      </svg>
      <strong className="text-[1rem] text-fg">{t('ui_not_found')}</strong>
      <p id="notfound-detail" className="m-0 max-w-136">
        {a?.missing ? t('ui_link_missing', a.path ?? '') : t('ui_not_found_detail', slug)}
      </p>
      <p className="m-0">
        <Button id="notfound-back" href={back}>
          <BackChevronIcon />
          {t('ui_to_page_list')}
        </Button>
      </p>
    </div>
  );
}
