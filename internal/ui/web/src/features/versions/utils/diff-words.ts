import { t } from '@/lib/t';
import type { MessageKey } from '@/types/messages.gen';
import { rawHref } from '@/utils/page-urls';

// What /api/pages/{slug}/diff answers (pagediff.Result with the pair): the counts, the sections of the later version
// with their state and the place of their first mark, and the places in the order of the page
export interface Span {
  op: string;
  text: string;
}
export type SectionState = 'same' | 'changed' | 'added' | 'removed';
export interface Section {
  state: SectionState;
  level: number;
  heading: Span[];
  place?: number;
}
export interface Counts {
  changed: number;
  added: number;
  removed: number;
  same: number;
}
export interface DiffAnswer {
  from: { n: number };
  to: { n: number };
  key: string;
  counts: Counts;
  sections: Section[];
  places: number[];
}

// countParts are the counts in words: a class, the number and its words; none added or removed are left out. All the
// same is one sentence alone (null)
export function countParts(c: Counts): { cls: string; n: number; word: MessageKey }[] | null {
  if (c.changed + c.added + c.removed === 0) return null;
  const all: { cls: string; n: number; word: MessageKey }[] = [
    { cls: 'c', n: c.changed, word: 'ui_diff_changed_n' },
    { cls: 'a', n: c.added, word: 'ui_diff_added_n' },
    { cls: 'r', n: c.removed, word: 'ui_diff_removed_n' },
    { cls: 's', n: c.same, word: 'ui_diff_same_n' },
  ];
  return all.filter((x) => x.n > 0 || (x.cls !== 'a' && x.cls !== 'r'));
}

// sectionName is a section's heading in words: a removed one's by its marked words (all deleted, pagediff's whole), any
// other's without what was deleted; the page's top for none
export function sectionName(s: Section): string {
  const skip = s.state === 'removed' ? '' : 'del';
  return (
    s.heading
      .filter((x) => x.op !== skip)
      .map((x) => x.text)
      .join('') || t('ui_diff_top')
  );
}

// levelOf is a section's level within the contents' three
export const levelOf = (s: Section): number => Math.min(Math.max(s.level, 1), 3);

// frameSrc is what the diff's iframe loads: the later version's page marked against the earlier, of the pair answered,
// at place at (-1 for its top, a move within it). None before an answer, nor of no page: the latest's page alone would
// be told read
export function frameSrc(slug: string, answer: DiffAnswer | null, at: number): string | null {
  if (!answer || !slug) return null;
  const marked = rawHref({ slug, v: answer.to.n, from: answer.from.n, key: answer.key });
  return at >= 0 ? `${marked}#floe-diff.${String(at)}` : marked;
}
