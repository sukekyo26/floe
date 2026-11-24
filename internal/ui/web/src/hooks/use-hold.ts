import { useCallback, useRef, useState, type RefObject } from 'react';
import { flushSync } from 'react-dom';

export interface Hold {
  held: boolean;
  // focus is what had the focus as the hold began, taken before the modal's fields are disabled (Chrome moves the
  // focus off a disabled field to the body), to be given back once it is let go
  focus: RefObject<Element | null>;
  // whileHeld holds the modal while write and the read after it go, and lets go however it ends (a throw is thrown on
  // once let go); the caller closes the modal after. It tells whether write ran: a second one while held does not run,
  // and its caller leaves the modal, its words and its focus as they are
  whileHeld: (write: () => Promise<void>) => Promise<boolean>;
  // holding tells the hold is on from the moment whileHeld is called, before the modal is drawn held: a key in that
  // task is stopped all the same (Dialog)
  holding: () => boolean;
}

// holdNext is a write asked for: it runs and holds unless a hold is on
export const holdNext = (holding: boolean): { holding: true; runs: boolean } => ({ holding: true, runs: !holding });

// useHold is the hold of one modal while it writes (Dialog's held). The hold is set at once in a ref, so that a second
// submit before the modal is drawn held does not run
// LIMIT: a write that never answers (the store's lock held by a stopped process) holds the modal till the page is
// loaded again. Let a write be called off if that comes to trouble someone
// whileHeld draws the hold at once (flushSync), so that the modal is held within the task it is called in, after an
// await too (the move's Yes): call it from an event's handler or what follows its await, never from a draw or an
// effect, where React cannot draw
export function useHold(): Hold {
  const [held, setHeld] = useState(false);
  const holding = useRef(false);
  const focus = useRef<Element | null>(null);
  const whileHeld = useCallback(async (write: () => Promise<void>): Promise<boolean> => {
    const next = holdNext(holding.current);
    if (!next.runs) return false;
    holding.current = next.holding;
    focus.current = document.activeElement;
    flushSync(() => {
      setHeld(true);
    });
    try {
      await write();
      return true;
    } finally {
      holding.current = false;
      setHeld(false);
    }
  }, []);
  const holdingNow = useCallback(() => holding.current, []);
  return { held, focus, whileHeld, holding: holdingNow };
}
