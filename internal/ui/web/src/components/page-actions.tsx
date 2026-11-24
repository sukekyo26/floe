import { barMenuId, prepareArchive, restoreFocus, rowMenuId } from '@/components/focus-places';
import { IconButton } from '@/components/icon-button';
import {
  ArchiveIcon,
  CloseIcon,
  CopyIcon,
  DiffIcon,
  DoneIcon,
  DownloadIcon,
  OpenIcon,
  StarIcon,
} from '@/components/icons';
import { PageMenu, useCopied, type Openers } from '@/components/page-menu';
import { t } from '@/lib/t';
import { pageActions as pageActionStore, type PageActions } from '@/stores/page-actions';
import type { Page } from '@/types/page';
import { canDiff, hasPage } from '@/utils/list-view';
import { baseName } from '@/utils/paths';
import { diffHref, downloadHref, pageHref, rawHref, type PageAt } from '@/utils/page-urls';

export interface RowActionsProps {
  readonly page: Page;
  readonly openers: Openers;
  readonly onPick?: () => void;
  readonly onRemoved?: () => void;
  readonly taskShown?: { project: string; name: string } | null;
  readonly actions?: PageActions;
}

// RowActions is what a row does beside its title: archive, star, and the rest in its menu (ui.md: a row shows the star
// and archive)
export function RowActions({
  page: a,
  openers,
  onPick,
  onRemoved,
  taskShown = null,
  actions = pageActionStore,
}: RowActionsProps) {
  const title = a.title ?? a.slug;
  return (
    <>
      <IconButton
        size="row"
        testid="archive"
        on={a.archived}
        title={t(a.archived ? 'ui_unarchive' : 'ui_archive')}
        label={t(a.archived ? 'ui_unarchive_named' : 'ui_archive_named', title)}
        // From the keys, its way back takes the focus: the toasts are the far end of the page from a row
        onClick={(e) =>
          void actions.setArchived(a, !a.archived, {
            focus: e.detail === 0,
            prepare: prepareArchive,
            restoreFocus: restoreFocus(a.slug),
          })
        }
      >
        <ArchiveIcon />
      </IconButton>
      <IconButton
        size="row"
        testid="star"
        tone="star"
        pressed={Boolean(a.favorite)}
        title={t('ui_favorite')}
        label={t('ui_favorite_named', title)}
        onClick={() => void actions.toggleFavorite(a)}
      >
        <StarIcon />
      </IconButton>
      <PageMenu
        page={a}
        at={{ slug: a.slug }}
        place="row"
        menuId={rowMenuId(a.slug)}
        openers={openers}
        {...(onPick ? { onPick } : {})}
        {...(onRemoved ? { onRemoved } : {})}
        taskShown={taskShown}
        actions={actions}
      />
    </>
  );
}

export interface BarActionsProps {
  // id is the bar's own: read-actions beside the list, frame-actions in the frame, find-actions in the search's foot
  readonly id: string;
  readonly place: 'list' | 'frame' | 'find';
  readonly page: Page;
  // at is what of the page the bar acts on: a folder's page (file), a version
  readonly at: PageAt;
  // diffing: the frame shows the diff, whose bar closes it in the place of the button that opened it
  readonly diffing?: boolean;
  // onDiffOpen is what opening the diff does first beside the list (the list's order goes on with j and k)
  readonly onDiffOpen?: () => void;
  readonly openers: Openers;
  readonly onRemoved?: () => void;
  readonly taskShown?: { project: string; name: string } | null;
  readonly actions?: PageActions;
}

// BarActions is the bar over a page: its diff, a folder's page's path, download, open, archive, star and menu. A
// folder's page holds no state of its own: no menu, and what the bar has stays there. In the frame, under 40rem, a bar
// with a menu folds download, open, archive and star into it
export function BarActions({
  id,
  place,
  page: a,
  at,
  diffing = false,
  onDiffOpen,
  openers,
  onRemoved,
  taskShown = null,
  actions = pageActionStore,
}: BarActionsProps) {
  const [copied, setCopied] = useCopied();
  const file = at.file ?? '';
  const title = a.title ?? a.slug;
  const folds = place === 'frame' && !file ? 'wide' : 'shown';
  return (
    // The search's foot tells whose its buttons are
    <span id={id} data-slug={place === 'find' ? a.slug : undefined} className="flex items-center gap-0.5">
      <IconButton
        testid="diff-open"
        on={diffing}
        hidden={!diffing && (Boolean(file) || !canDiff(a))}
        href={diffing ? pageHref({ slug: a.slug }) : diffHref(a.slug, at.v, a.version)}
        label={t(diffing ? 'ui_diff_close_title' : 'ui_diff_title')}
        onClick={place === 'list' ? onDiffOpen : undefined}
      >
        {diffing ? <CloseIcon /> : <DiffIcon />}
      </IconButton>
      <IconButton
        testid="copy"
        hidden={!file}
        label={t('ui_copy_path')}
        title={t(copied ? 'ui_copied' : 'ui_copy_path')}
        onClick={() => {
          void actions.copyPath(`${a.path ?? ''}/${file}`).then((ok) => {
            if (ok) setCopied();
          });
        }}
      >
        {copied ? <DoneIcon /> : <CopyIcon />}
      </IconButton>
      <IconButton
        testid="download"
        display={folds}
        hidden={Boolean(file) || !hasPage(a)}
        href={downloadHref(at)}
        download
        title={t('ui_download')}
        label={t('ui_download_named', title)}
      >
        <DownloadIcon />
      </IconButton>
      <IconButton
        testid="open"
        display={folds}
        hidden={file ? Boolean(a.missing) : !hasPage(a)}
        href={rawHref(at)}
        newTab
        title={t('ui_open_raw')}
        label={t('ui_open_raw_named', file ? baseName(file) : title)}
      >
        <OpenIcon />
      </IconButton>
      <IconButton
        testid="archive"
        display={folds}
        hidden={Boolean(file)}
        on={a.archived}
        title={t(a.archived ? 'ui_unarchive' : 'ui_archive')}
        label={t(a.archived ? 'ui_unarchive_named' : 'ui_archive_named', title)}
        onClick={() =>
          void actions.setArchived(a, !a.archived, { prepare: prepareArchive, restoreFocus: restoreFocus(a.slug) })
        }
      >
        <ArchiveIcon />
      </IconButton>
      <IconButton
        testid="star"
        tone="star"
        display={folds}
        hidden={Boolean(file)}
        pressed={Boolean(a.favorite)}
        title={t('ui_favorite')}
        label={t('ui_favorite_named', title)}
        onClick={() => void actions.toggleFavorite(a)}
      >
        <StarIcon />
      </IconButton>
      <span data-testid="more-slot" className="contents">
        {file ? null : (
          <PageMenu
            page={a}
            at={at}
            place={place}
            menuId={barMenuId(id)}
            openers={openers}
            {...(onRemoved ? { onRemoved } : {})}
            taskShown={taskShown}
            actions={actions}
          />
        )}
      </span>
    </span>
  );
}
