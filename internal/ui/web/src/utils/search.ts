import { matches } from '@/utils/list-view';
import type { Page } from '@/types/page';

// searched are the entries of among that the words of a search field match
export const searched = (words: string, among: readonly Page[]): Page[] => {
  const q = words.trim().toLowerCase();
  return among.filter((a) => matches(a, q));
};
