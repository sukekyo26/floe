import { TextLink } from '@/components/text-link';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { pageNote as sharedNote, type PageNote as Notes } from '@/stores/page-note';
import { diffHref } from '@/utils/page-urls';

interface PageNoteProps {
  // slug is the page shown: words of another are not drawn
  readonly slug: string | undefined;
  // onGo is what the link does first beside the list (the list's order goes on with j and k)
  readonly onGo?: () => void;
  readonly note?: Notes;
}

// PageNote is the words over a page newly at another version than the one read, with the way to its diff
export function PageNote({ slug, onGo, note = sharedNote }: PageNoteProps) {
  const said = useStore(note.note);
  if (!said || said.slug !== slug) return null;
  return (
    <div
      data-testid="page-note"
      className="flex flex-none flex-wrap items-center gap-x-3 gap-y-1 border-b border-line bg-new-soft px-4 py-1.75 text-small text-fg narrow:px-3"
    >
      {t('ui_diff_notice', said.read, said.changed)}
      <TextLink tone="new" href={diffHref(said.slug)} onClick={onGo}>
        {t('ui_diff_notice_go')}
      </TextLink>
    </div>
  );
}
