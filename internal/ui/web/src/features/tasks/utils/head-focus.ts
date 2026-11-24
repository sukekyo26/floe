// What of a task's head had the focus just before it was drawn anew: its state's choices, a button of the parent's
// part, or a row of its family card (its link, or the button that takes it off)
export type HeadFocused = { on: 'state' } | { on: 'parent' } | { on: 'kin'; task: string; unlink: boolean };

export interface HeadNow {
  // stateHidden: the choices hidden (archived); parentHidden: the parent's button that had the focus hidden; rebuilt:
  // the card's rows made anew, which took the focus with them
  stateHidden: boolean;
  parentHidden: boolean;
  rebuilt: boolean;
}

// Where the focus goes then: the archive button, the parent's part's button shown now, or the same row's same thing
export type HeadTarget = { to: 'archive' } | { to: 'part-up' } | { to: 'kin'; task: string; unlink: boolean };

// headFocus is where the focus goes once the head is drawn anew: the choices hidden, to the archive button beside the
// word; the parent's button gone, to the one there now; the card made anew, to the same row's link or button. null
// where it may stay
export function headFocus(was: HeadFocused | null, now: HeadNow): HeadTarget | null {
  if (was?.on === 'state' && now.stateHidden) return { to: 'archive' };
  if (was?.on === 'parent' && now.parentHidden) return { to: 'part-up' };
  if (was?.on === 'kin' && now.rebuilt) return { to: 'kin', task: was.task, unlink: was.unlink };
  return null;
}

export interface Drawn {
  state: string;
  rev: number;
}

// What a task's state's choices show: the state and rev drawn (data-state, its colour, the rev a change is sent of), the
// value chosen while it is sent (null when nothing is), and what it was last taken from: the task's state and rev, and
// the reads ended then
export interface StateShown {
  state: string;
  rev: number;
  chosen: string | null;
  from: Drawn;
  reads: number;
}

export const firstShown = (task: Drawn, reads: number): StateShown => ({
  state: task.state,
  rev: task.rev,
  chosen: null,
  from: { state: task.state, rev: task.rev },
  reads,
});

// stateShown is what the choices show as the task or the reads change: held while they are open or a change is sent;
// else taken from the task as it changes, and at each read ended (a read of the same words changes no props). A change
// written keeps its from, so that the props of before the read after it are not taken: the store has the answer's row
// by then
export function stateShown(now: StateShown, task: Drawn, reads: number, open: boolean): StateShown {
  if (open || now.chosen !== null) return now;
  if (task.state === now.from.state && task.rev === now.from.rev && reads === now.reads) return now;
  return firstShown(task, reads);
}
