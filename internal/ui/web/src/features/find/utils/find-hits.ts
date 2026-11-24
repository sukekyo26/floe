import type { Page } from '@/types/page';
import type { MessageKey } from '@/types/messages.gen';
import { FIND_ROWS, searched } from '@/utils/search';

// LIMIT: the files in a folder are not searched, as what is in a folder is read only where it is open

// findHits are the entries the words match, the archived after the rest (as they would drown what is in use), each
// kept in the list's order
export const findHits = (words: string, entries: readonly Page[]): Page[] =>
  searched(words, entries).toSorted((x, y) => Number(Boolean(x.archived)) - Number(Boolean(y.archived)));

// findCount is what the line over the rows says of them: cut to the first rows, how many, or the newest first
export function findCount(words: string, total: number): { key: MessageKey; n: number } {
  if (total > FIND_ROWS) return { key: 'ui_find_cut', n: FIND_ROWS };
  if (words.trim() || total === 0) return { key: 'ui_find_n', n: total };
  return { key: 'ui_find_newest', n: total };
}
