import type { CSSProperties, ReactNode } from 'react';

import { t } from '@/lib/t';
import type { Page } from '@/types/page';
import type { CssVars } from '@/utils/css-vars';
import { unseen } from '@/utils/list-view';
import { projectColor } from '@/utils/project-color';

interface RowSubProps {
  readonly page: Page;
  // ranks is the projects' order of colours (projectOrder)
  readonly ranks: ReadonlyMap<string, number>;
  // middle is what the caller puts after the project: the search, an archived page's badge
  readonly middle?: ReactNode;
}

const PILL = 'rounded-full px-2 whitespace-nowrap';

// RowSub is the line under a row's title (ui.html drawSub): a new version, the kind, the project in its colour, what
// the caller adds, then a link's path or that its file is gone. The tags are the list row's own (P5)
export function RowSub({ page: a, ranks, middle }: RowSubProps) {
  const project = a.project ?? '';
  const tint: CssVars = { '--c': projectColor(project, ranks) };
  return (
    <>
      {unseen(a) && a.news === 'version' ? (
        <span
          data-testid="ver"
          className={`${PILL} bg-new-soft font-mono text-label leading-normal font-bold text-new`}
        >
          {t('ui_new_version', a.version ?? 0)}
        </span>
      ) : null}
      {a.folder || a.path || a.template ? (
        <span data-testid="kindof" className="rounded-small border border-line px-1.25 whitespace-nowrap text-muted">
          {a.folder ? t('ui_kind_folder') : a.path ? t('ui_kind_of_link', a.template ?? 'HTML') : a.template}
        </span>
      ) : null}
      {ranks.has(project) ? (
        // The one colour mixed outside :root (ui.md manage-colours): --c is each row's
        <span
          data-testid="proj"
          style={tint as CSSProperties}
          className={`${PILL} inline-flex max-w-40 items-center gap-1.25 bg-(--tint) font-medium text-(--tint-ink) [--tint-ink:color-mix(in_srgb,var(--c)_50%,var(--color-fg))] [--tint:color-mix(in_srgb,var(--c)_16%,var(--color-panel))] before:size-1.75 before:flex-none before:rounded-full before:bg-(--c) before:content-['']`}
        >
          <span data-testid="proj-name" className="min-w-0 truncate">
            {project}
          </span>
        </span>
      ) : null}
      {middle}
      {a.missing ? (
        <span data-testid="miss" className={`${PILL} bg-chg-soft text-chg`}>
          {t('ui_file_missing')}
        </span>
      ) : a.path ? (
        // Cut at its head, so that the file's name stays: the box runs right to left, its bdi keeping the path's order
        <span
          data-testid="path"
          className="min-w-0 flex-[1_1_8rem] truncate text-left font-mono text-label [direction:rtl]"
        >
          <bdi dir="ltr">{a.path}</bdi>
        </span>
      ) : null}
    </>
  );
}
