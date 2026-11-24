import type { CSSProperties, KeyboardEventHandler, MouseEventHandler } from 'react';

import { ChevronIcon, CopyIcon, DoneIcon, OpenIcon, TreeDirIcon, TreePageIcon } from '@/components/icons';
import { IconButton } from '@/components/icon-button';
import { useCopied } from '@/components/page-menu';
import { t } from '@/lib/t';
import { Ago } from '@/components/ago';
import { fmt } from '@/lib/time';
import { pageActions as pageActionStore, type PageActions } from '@/stores/page-actions';
import type { CssVars } from '@/utils/css-vars';
import { pageHref, rawHref } from '@/utils/page-urls';
import { baseName } from '@/utils/paths';
import type { Listing } from '@/types/page';
import type { TreeItem } from '@/utils/tree-items';

export interface TreeRowProps {
  readonly item: TreeItem;
  // folder is the folder the tree is of: its slug, and its path on disk that a page's path is copied under
  readonly folder: { readonly slug: string; readonly path: string };
  // picked: the page open beside the list (a page's row alone); archived: of a folder archived, dimmed (a task's page)
  readonly picked?: boolean;
  readonly archived?: boolean;
  // onToggle opens or closes a directory's row
  readonly onToggle?: (path: string) => void;
  readonly onTitleClick?: MouseEventHandler<HTMLAnchorElement>;
  readonly onTitleKeyDown?: KeyboardEventHandler<HTMLAnchorElement>;
  readonly actions?: PageActions;
}

// treeNote is what a directory says under its rows: why it was not read, that it was cut, that it is empty (the list's
// tree and the frame's)
export function treeNote(l: Listing | Error): string {
  if (l instanceof Error) return t('ui_folder_failed', l.message);
  const shown = l.dirs.length + l.files.length;
  if (l.more) return t('ui_folder_more', shown + l.more, shown);
  return shown === 0 ? t('ui_folder_empty') : '';
}

// What a tree's row begins with, in the list's and the frame's: a directory's fold, which turns open, and the icon of a
// directory or a page. The fold's room is kept in a page's row and a note's, so that the names line up under a
// directory's
export const FOLD =
  'me-[0.35rem] inline-block size-3.5 align-[-2px] text-muted transition-transform duration-150 [&>svg]:size-3.5';
export const KIND = 'me-[0.4rem] inline-block size-4 align-[-3px] text-muted [&>svg]:size-4';
const TITLE =
  'block truncate font-medium text-fg no-underline after:absolute after:inset-0 focus-visible:outline-none group-data-archived/tree:text-muted';

// TreeRow is a row of a folder's tree in the list: a directory that opens and closes, a page of it with its way to open
// alone and to copy its path, or a directory's note. A folder open and its rows are one card: this row has the sides,
// the last of them the foot
export function TreeRow({
  item,
  folder,
  picked = false,
  archived = false,
  onToggle,
  onTitleClick,
  onTitleKeyDown,
  actions = pageActionStore,
}: TreeRowProps) {
  const [copied, setCopied] = useCopied();
  const style: CssVars = { '--d': item.d };
  const at = item.kind === 'page' ? { slug: folder.slug, file: item.file } : null;
  const name = item.kind === 'page' ? baseName(item.file) : '';
  return (
    <li
      data-testid="tree"
      data-key={item.key}
      data-d={item.d}
      data-slug={folder.slug}
      data-file={item.kind === 'page' ? item.file : undefined}
      aria-current={picked || undefined}
      data-archived={archived || undefined}
      style={style as CSSProperties}
      className="group/tree relative flex items-center gap-2 border border-y-0 border-line bg-panel py-[0.2rem] ps-[calc(1.5rem+(var(--d)-1)*1.1rem)] pe-4 text-small transition-colors duration-150 not-has-[+li[data-testid=tree]]:rounded-b-menu not-has-[+li[data-testid=tree]]:border-b not-has-[+li[data-testid=tree]]:shadow-sm hover:bg-hover has-[[data-testid=title]:focus-visible]:outline-2 has-[[data-testid=title]:focus-visible]:-outline-offset-2 has-[[data-testid=title]:focus-visible]:outline-faint aria-current:bg-new-soft"
    >
      <div className={`min-w-0 flex-1 ${item.kind === 'note' ? 'ps-[calc(0.875rem+0.35rem)] text-muted' : ''}`}>
        {item.kind === 'dir' ? (
          <button
            type="button"
            data-testid="title"
            aria-expanded={item.open}
            title={item.path}
            onClick={() => onToggle?.(item.path)}
            className={`${TITLE} max-w-full cursor-pointer border-0 bg-transparent p-0 text-start`}
          >
            <span className={`${FOLD} ${item.open ? 'rotate-90' : ''}`}>
              <ChevronIcon />
            </span>
            <span className={KIND}>
              <TreeDirIcon />
            </span>
            {baseName(item.path)}
          </button>
        ) : item.kind === 'page' && at ? (
          <a
            data-testid="title"
            href={pageHref(at)}
            title={`${item.file}\n${t('ui_updated_at', fmt(item.updatedAt))}`}
            onClick={onTitleClick}
            onKeyDown={onTitleKeyDown}
            className={TITLE}
          >
            <span className={`${FOLD} invisible`} />
            <span className={KIND}>
              <TreePageIcon />
            </span>
            {name}
          </a>
        ) : item.kind === 'note' ? (
          treeNote(item.listing)
        ) : null}
      </div>
      <time
        dateTime={item.kind === 'page' ? item.updatedAt : undefined}
        className="flex-none font-mono text-label leading-none whitespace-nowrap text-muted"
      >
        {item.kind === 'page' ? <Ago iso={item.updatedAt} /> : null}
      </time>
      <div
        data-testid="actions"
        className="relative z-1 flex flex-none items-center gap-0.5 can-hover:[&>*]:opacity-0 can-hover:[&>*]:transition-opacity can-hover:[&>*]:duration-120 can-hover:group-focus-within/tree:[&>*]:opacity-100 can-hover:group-hover/tree:[&>*]:opacity-100 can-hover:group-aria-current/tree:[&>*]:opacity-100"
      >
        {at ? (
          <>
            <IconButton
              testid="open"
              size="row"
              href={rawHref(at)}
              newTab
              label={t('ui_open_raw_named', name)}
              title={t('ui_open_raw')}
            >
              <OpenIcon />
            </IconButton>
            <IconButton
              testid="copy"
              size="row"
              label={t('ui_copy_path_named', name)}
              title={copied ? t('ui_copied') : t('ui_copy_path')}
              onClick={() => {
                void actions.copyPath(`${folder.path}/${at.file}`).then((ok) => {
                  if (ok) setCopied();
                });
              }}
            >
              {copied ? <DoneIcon /> : <CopyIcon />}
            </IconButton>
          </>
        ) : null}
      </div>
    </li>
  );
}
