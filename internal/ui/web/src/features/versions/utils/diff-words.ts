import { t } from '@/lib/t';
import type { MessageKey } from '@/types/messages.gen';

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
