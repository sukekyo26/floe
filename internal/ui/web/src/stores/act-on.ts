import { notice as pageNotice, type Notice } from '@/stores/notice';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';

// What a write to many pages says: done(n of them written), failed(n of them refused, the first reason)
export interface ActWords {
  done: (n: number) => string;
  failed: (n: number, why: string) => string;
}

// ActOn writes act to each of slugs, says how many were written (a toast) and how many failed (the error line, taken
// back when none did), and reads the list after; settled(reasons) comes before that read
export type ActOn = (
  act: (slug: string) => Promise<string>,
  words: ActWords,
  slugs: string[],
  settled?: (reasons: string[]) => void,
) => Promise<void>;

interface Deps {
  toasts?: Toasts;
  notice?: Notice;
  pages?: Pages;
}

// createActOn writes the tags of a page or of the selected, and archives or deletes the selected
export function createActOn({ toasts = pageToasts, notice = pageNotice, pages = pagePages }: Deps = {}): ActOn {
  return async (act, { done, failed }, slugs, settled) => {
    const reasons = await Promise.all(slugs.map(act));
    settled?.(reasons);
    const why = reasons.filter(Boolean);
    if (why.length < slugs.length) toasts.toast(done(slugs.length - why.length));
    notice.say(why.length ? failed(why.length, String(why[0])) : '');
    await pages.load();
  };
}

export const actOn = createActOn();
