// What a modal shows a task by: what it was opened for, the project and name typed, and the 409s it was refused by
export interface SeenKey {
  open: object | null;
  project: string;
  name: string;
  renews: number;
}

// seenAgain tells when a modal finds again the task it shows: as it opens, as the project or the name typed changes,
// and as a write is refused as stale. A read of the list alone does not, so that a save sends what was shown: a task
// made or written since is refused, not written over unseen
export const seenAgain = (was: SeenKey | null, now: SeenKey): boolean =>
  was === null ||
  was.open !== now.open ||
  was.project !== now.project ||
  was.name.trim() !== now.name.trim() ||
  was.renews !== now.renews;
