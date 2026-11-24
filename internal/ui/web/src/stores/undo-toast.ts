import { t } from '@/lib/t';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';

// How a change is taken back: undo writes what was there before and answers { ok, text }, or undefined where the press
// is dropped (pressOnce); done is where the focus goes once it is back, away where Escape on it leaves the focus
export interface Undo {
  undo: () => Promise<{ ok: boolean; text: string } | undefined>;
  done: () => void;
  away: () => void;
}

export interface UndoOptions {
  // focus: the change was made from the keys, and its way back takes the focus
  focus?: boolean;
}

export type UndoToast = (said: string, how: Undo, options?: UndoOptions) => number;

interface Deps {
  toasts?: Toasts;
  notice?: Notice;
  pages?: Pages;
}

// createUndoToast makes the toast with a way back: pressed, it holds, writes what was there before and goes whatever
// comes; a refusal (written meanwhile, say) is said as the store says it
export function createUndoToast({ toasts = pageToasts, notice = pageNotice, pages = pagePages }: Deps = {}): UndoToast {
  return (said, how, { focus = false } = {}) => {
    const id = toasts.toast(said, {
      ms: 8000,
      focus,
      button: {
        label: t('ui_undo'),
        undo: true,
        holds: true,
        onPress: async () => {
          const res = await how.undo();
          if (!res) return;
          toasts.remove(id);
          if (!res.ok) {
            notice.say(t('ui_save_failed', res.text));
            return;
          }
          toasts.toast(t('ui_undone'));
          await pages.load();
          how.done();
        },
        onEscape: () => {
          toasts.remove(id);
          how.away();
        },
      },
    });
    return id;
  };
}

export const undoToast = createUndoToast();
