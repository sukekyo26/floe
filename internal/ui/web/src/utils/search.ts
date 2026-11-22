import { matches } from '@/utils/list-view';
import type { Page } from '@/types/page';

// FIND_ROWS is the most rows a search modal shows (the search, and Add a page of a task)
// LIMIT: the first FIND_ROWS rows only, which the search says. Words narrow them; show more if a search comes to need them
export const FIND_ROWS = 50;

// searched are the entries of among that the words of a search field match
export const searched = (words: string, among: readonly Page[]): Page[] => {
  const q = words.trim().toLowerCase();
  return among.filter((a) => matches(a, q));
};
