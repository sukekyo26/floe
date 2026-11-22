import { verRest, type VersionList } from '@/features/versions/utils/version-words';
import type { OpenAt } from '@/stores/open-page';
import type { Page } from '@/types/page';
import { canDiff } from '@/utils/list-view';

// The choice of versions as drawn: hidden (a page of one version, its own chosen), old (a version other than the
// latest), the value chosen, the numbers listed, and the options newest first
export interface SelectView {
  hidden: boolean;
  old: boolean;
  value: number;
  numbers: string;
  options: { n: number; rest: string }[];
}

// The words over a page not the latest: the version asked for gone, or one older shown (n of time at, last the latest)
export type NoteView = { kind: 'gone' } | { kind: 'old'; n: number; at: string; last: number } | null;

export interface VersionView {
  select: SelectView | null;
  note: NoteView;
}

// versionsWanted tells whether the versions of the page at are read: of a stored page opened at a version, or one that
// has versions to choose; never of a folder's file, nor on the diff, which has choices of its own
export const versionsWanted = (at: OpenAt | null, page: Page | null, diff: boolean): boolean =>
  at !== null && page !== null && !at.file && !diff && (canDiff(page) || at.v > 0);

// versionLeave tells whether the version at shows has to give way to the latest: it has come to be the latest, or it is
// no version listed (gone); null while it is one of the older ones, or none is chosen
export function versionLeave(at: OpenAt, { versions }: VersionList): { gone: boolean } | null {
  if (!at.v) return null;
  if (at.v === versions.at(-1)?.n) return { gone: false };
  return versions.some((v) => v.n === at.v) ? null : { gone: true };
}

// versionView is what the choice and the words show of at, page being its entry in the list of pages. list is the
// versions read of this page, null while none are (another page's are never passed): until then the choice is hidden,
// and so it is while the version shown is to give way. The words of a version gone outlast the choice
export function versionView({
  at,
  page,
  list,
  gone,
  diff,
}: {
  at: OpenAt | null;
  page: Page | null;
  list: VersionList | null;
  gone: boolean;
  diff: boolean;
}): VersionView {
  const goneNote: NoteView = gone ? { kind: 'gone' } : null;
  const last = list?.versions.at(-1);
  if (!list || !last || !at || !page || !versionsWanted(at, page, diff) || versionLeave(at, list)) {
    return { select: null, note: goneNote };
  }
  const shown = list.versions.find((v) => v.n === at.v);
  return {
    select: {
      hidden: !canDiff(page),
      old: at.v > 0,
      value: at.v || last.n,
      numbers: list.versions.map((v) => v.n).join(),
      options: list.versions.toReversed().map((v) => ({ n: v.n, rest: verRest(v, list) })),
    },
    note: goneNote ?? (shown ? { kind: 'old', n: shown.n, at: shown.at, last: last.n } : null),
  };
}
