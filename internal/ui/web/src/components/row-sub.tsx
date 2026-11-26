import type { ReactNode } from 'react';

import { PILL, ProjectPill } from '@/components/project-pill';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';
import { unseen } from '@/utils/list-view';
import { projectColor } from '@/utils/project-color';

interface RowSubProps {
  readonly page: Page;
  // ranks is the projects' order of colours (projectOrder)
  readonly ranks: ReadonlyMap<string, number>;
  // middle is what the caller puts after the project: the search, an archived page's badge
  readonly middle?: ReactNode;
}

// NEW_PILL is the pill of what is new on a row: a page's new version, a task's unread pages
export const NEW_PILL = `${PILL} bg-new-soft font-mono text-label leading-normal font-bold text-new`;

// RowSub is the line under a row's title: a new version, the kind, the project in its colour, what the caller adds,
// then a link's path or that its file is gone. The tags are the list row's own (P5)
export function RowSub({ page: a, ranks, middle }: RowSubProps) {
  const project = a.project ?? '';
  return (
    <>
      {unseen(a) && a.news === 'version' ? (
        <span data-testid="ver" className={NEW_PILL}>
          {t('ui_new_version', a.version ?? 0)}
        </span>
      ) : null}
      {a.folder || a.path || a.template ? (
        <span data-testid="kindof" className="truncate rounded-small border border-line px-1.25 text-muted">
          {a.folder ? t('ui_kind_folder') : a.path ? t('ui_kind_of_link', a.template ?? 'HTML') : a.template}
        </span>
      ) : null}
      {ranks.has(project) ? <ProjectPill word={project} color={projectColor(project, ranks)} /> : null}
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
