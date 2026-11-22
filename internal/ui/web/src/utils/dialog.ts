import { composing, type KeyPress } from '@/utils/keys';

// Where a press on a modal went down: on its backdrop (the dialog itself; its body fills it) or not, and whether the
// modal was held then
export interface PressDown {
  onDialog: boolean;
  held: boolean;
}

// backdropCloses tells a click that closes a modal: one that went down on its backdrop while it was not held, and came
// up there too. A click lands on the dialog also when it went down in a field and up outside, or down while held and up
// once let go
export const backdropCloses = (down: PressDown | null, upOnDialog: boolean): boolean =>
  down !== null && down.onDialog && !down.held && upOnDialog;

// heldKey tells a key that goes on to the browser while a modal is held by a write: those with a modifier and the
// function keys (a page held by a write that never answers can still be loaded again), and an input method's
export const heldKey = (e: KeyPress & Pick<KeyboardEvent, 'isComposing' | 'keyCode'>): boolean =>
  e.ctrlKey || e.metaKey || e.altKey || /^F\d+$/.test(e.key) || composing(e);

// stopsKey tells a key a held modal stops: held as drawn, or holding already in the task the write began in (its draw
// not yet done), unless heldKey lets it go on
export const stopsKey = (held: boolean, holding: boolean, passes: boolean): boolean => (held || holding) && !passes;
