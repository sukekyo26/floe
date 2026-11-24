import { TextLink } from '@/components/text-link';
import { versions as sharedVersions, type Versions } from '@/features/versions/stores/versions';
import { verMonth } from '@/features/versions/utils/version-words';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import type { OpenAt } from '@/stores/open-page';
import type { Page } from '@/types/page';
import { diffHref, pageHref } from '@/utils/page-urls';

interface VersionNoteProps {
  readonly at: OpenAt | null;
  readonly page: Page | null;
  // onGo is what the way to the diff does first beside the list (the list's order goes on with j and k)
  readonly onGo?: () => void;
  readonly versions?: Versions;
}

// VersionNote is the words over a page not the latest: the version shown and the ways to the latest and to their diff,
// or that the version asked for is gone. It goes over the words of a page newly at another version (PageNote), each on
// its own turns
export function VersionNote({ at, page, onGo, versions = sharedVersions }: VersionNoteProps) {
  useStore(versions.state);
  const note = versions.view(at, page).note;
  if (!note || !at) return null;
  return (
    <output
      data-testid="ver-note"
      className="flex flex-none flex-wrap items-center gap-x-3 gap-y-1 border-b border-line bg-chg-soft px-4 py-1.75 text-small text-fg narrow:px-3"
    >
      {note.kind === 'gone' ? (
        t('ui_version_gone')
      ) : (
        <>
          <span>{t('ui_version_old', note.n, verMonth(note.at), note.last)}</span>
          <span className="ms-auto flex flex-wrap gap-x-3.5 gap-y-1 narrow:ms-0">
            <TextLink
              tone="chg"
              href={pageHref({ slug: at.slug })}
              onClick={(e) => {
                if (e.button || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
                e.preventDefault();
                versions.pick(note.last, { focus: true, held: false });
              }}
            >
              {t('ui_version_latest')}
            </TextLink>
            <TextLink tone="chg" href={diffHref(at.slug, note.n, note.last)} onClick={onGo}>
              {t('ui_version_diff')}
            </TextLink>
          </span>
        </>
      )}
    </output>
  );
}
