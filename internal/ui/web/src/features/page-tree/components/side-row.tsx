import type { CSSProperties } from 'react';

import { ChevronIcon, TreeDirIcon, TreePageIcon } from '@/components/icons';
import { FOLD, KIND, treeNote } from '@/components/tree-row';
import type { CssVars } from '@/utils/css-vars';
import { pageHref } from '@/utils/page-urls';
import { baseName } from '@/utils/paths';
import type { TreeItem } from '@/utils/tree-items';

const ROW =
  'block w-full truncate rounded-small border-0 bg-transparent py-[0.22rem] ps-[calc(0.3rem+var(--d)*1.1rem)] pe-2 text-start text-fg no-underline hover:bg-hover';

export interface SideRowProps {
  readonly item: TreeItem;
  // slug is the folder's; file the page framed, its row the current one
  readonly slug: string;
  readonly file: string;
  readonly onToggle: (dir: string) => void;
}

// SideRow is a row of the frame's tree: a directory that opens and closes, a page of it that the frame moves to, as any
// link, or a directory's note. A row is the tree's child itself, with nothing beside it
export function SideRow({ item, slug, file, onToggle }: SideRowProps) {
  const style = { '--d': item.d } as CssVars as CSSProperties;
  if (item.kind === 'dir')
    return (
      <button
        type="button"
        aria-expanded={item.open}
        title={item.path}
        style={style}
        onClick={() => {
          onToggle(item.path);
        }}
        className={`${ROW} cursor-pointer font-[650]`}
      >
        <span className={`${FOLD} ${item.open ? 'rotate-90' : ''}`}>
          <ChevronIcon />
        </span>
        <span className={KIND}>
          <TreeDirIcon />
        </span>
        {baseName(item.path)}
      </button>
    );
  if (item.kind === 'page')
    return (
      <a
        href={pageHref({ slug, file: item.file })}
        title={item.file}
        aria-current={item.file === file ? 'page' : undefined}
        style={style}
        className={`${ROW} aria-[current=page]:bg-sel aria-[current=page]:font-bold`}
      >
        <span className={`${FOLD} invisible`} />
        <span className={KIND}>
          <TreePageIcon />
        </span>
        {baseName(item.file)}
      </a>
    );
  return (
    <p
      style={style}
      className="m-0 py-[0.22rem] ps-[calc(0.3rem+var(--d)*1.1rem+0.875rem+0.35rem)] pe-2 text-small text-muted"
    >
      {treeNote(item.listing)}
    </p>
  );
}
