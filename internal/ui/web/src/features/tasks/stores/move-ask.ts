import { moveLead, movesFrom } from '@/features/tasks/utils/task-set';
import { createStore, type Store } from '@/lib/store';
import { pages as pagePages } from '@/stores/pages';
import type { Page } from '@/types/page';

// What the move's question says, while it is asked
export interface MoveAsking {
  lead: string;
}

export interface MoveAsk {
  asking: Store<MoveAsking | null>;
  // ask tells whether to put page on the task to: asked when the list now has it on another task (the page as the
  // modal opened, when the list has it not), and at once yes when it is on none or on that one
  ask: (page: Pick<Page, 'slug' | 'title' | 'task'>, to: string) => Promise<boolean>;
  // answer is the question's answer as it closes, however it closes; a second answer does nothing
  answer: (yes: boolean) => void;
}

// createMoveAsk is the question asked over Set the task or Add a page before a page on another task is moved: the modal
// it came from stays as it was
// LIMIT: a page put on another task since the list was read is moved unasked, which the toast after the write tells.
// Give page writes a rev if that comes to trouble someone
// entries is the list as read now
export function createMoveAsk({
  entries = () => pagePages.state.get().entries,
}: { entries?: () => readonly Page[] } = {}): MoveAsk {
  const asking = createStore<MoveAsking | null>(null);
  let answered: ((yes: boolean) => void) | null = null;
  return {
    asking,
    ask: (page, to) => {
      const listed = entries().find((a) => a.slug === page.slug) ?? page;
      if (!movesFrom(listed, to)) return Promise.resolve(true);
      asking.set({ lead: moveLead(listed, to) });
      return new Promise((resolve) => {
        answered = resolve;
      });
    },
    answer: (yes) => {
      const resolve = answered;
      answered = null;
      asking.set(null);
      resolve?.(yes);
    },
  };
}

export const moveAsk = createMoveAsk();
