// moved is list with name put before the row at before (list.length: last), as insertBefore puts a row
export function moved(list: readonly string[], name: string, before: number): string[] {
  const from = list.indexOf(name);
  if (from === -1) return [...list];
  const rest = list.filter((x) => x !== name);
  rest.splice(before > from ? before - 1 : before, 0, name);
  return rest;
}

// droppedTo is the rows once name is dropped before the row at before, or null where that leaves them as they were
// (dropped on the row just under it in its upper half, or the row just over it in its lower half): nothing to say
export function droppedTo(list: readonly string[], name: string, before: number): string[] | null {
  const next = moved(list, name, before);
  return next.join('\n') === list.join('\n') ? null : next;
}

// dropsAfter tells a drop at y goes after the row of top and height: in its lower half
export const dropsAfter = (top: number, height: number, y: number): boolean => y > top + height / 2;

// ends is which of a row's buttons move nothing: the first row's up, the last row's down
export const ends = (at: number, n: number): { up: boolean; down: boolean } => ({ up: at === 0, down: at === n - 1 });

// The way a row moves by its buttons
export type Step = 'up' | 'down';

// focusStep is the button that takes the focus once the row moved by pressed is at: the one pressed, or the other where
// the one pressed moves nothing more
export function focusStep(pressed: Step, at: number, n: number): Step {
  const end = ends(at, n);
  if (pressed === 'up' && end.up) return 'down';
  if (pressed === 'down' && end.down) return 'up';
  return pressed;
}

// nearDropped is the row that takes the focus once the one at is taken off, of n left: the one after it, which comes to
// its place, or the one before it; -1 for none
export const nearDropped = (at: number, n: number): number => (at < n ? at : n - 1);
