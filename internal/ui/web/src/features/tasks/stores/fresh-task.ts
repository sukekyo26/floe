import { createStore, type Store } from '@/lib/store';
import { listQuery, type ListQuery } from '@/stores/list-query';

export interface FreshTask {
  // key is the row of the task just made (data-fresh), null for none
  key: Store<string | null>;
  // mark marks the row of key, as the list is now
  mark: (key: string) => void;
  // follow lets the mark go as the list's project or view changes; it returns what stops it
  follow: () => () => void;
}

// createFreshTask is the mark of the task just made on the task list: it stays till another is made, or the list shows
// another project or view
export function createFreshTask(query: ListQuery = listQuery): FreshTask {
  const key = createStore<string | null>(null);
  let at = '';
  const where = () => {
    const { project, view } = query.query.get();
    return JSON.stringify([project, view]);
  };
  return {
    key,
    mark: (row) => {
      at = where();
      key.set(row);
    },
    follow: () =>
      query.query.subscribe(() => {
        if (key.get() !== null && where() !== at) key.set(null);
      }),
  };
}

export const freshTask = createFreshTask();
