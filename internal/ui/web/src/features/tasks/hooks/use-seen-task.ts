import { useCallback, useState } from 'react';

import { seenAgain, type SeenKey } from '@/features/tasks/utils/seen-task';
import type { TaskOf } from '@/types/task';

export interface SeenTask {
  // seen is the task of the project and name as last found, or undefined where there was none
  seen: TaskOf | undefined;
  // renew finds it again on the next draw: a write refused as stale (409), the list read again
  renew: () => void;
}

// useSeenTask is the task a modal shows of project and name, found in tasks only when seenAgain says (opened, the
// project or name changed, renew), held over the reads between
export function useSeenTask(open: object | null, project: string, name: string, tasks: readonly TaskOf[]): SeenTask {
  const [renews, setRenews] = useState(0);
  const key: SeenKey = { open, project, name, renews };
  const find = () => tasks.find((x) => x.project === project && x.name === name.trim());
  const [held, setHeld] = useState(() => ({ key, seen: find() }));
  let now = held;
  if (seenAgain(held.key, key)) {
    now = { key, seen: find() };
    setHeld(now);
  }
  const renew = useCallback(() => {
    setRenews((n) => n + 1);
  }, []);
  return { seen: now.seen, renew };
}
