import { useCallback, useRef, useState, type RefObject } from 'react';

export interface Hold {
  held: boolean;
  // focus is what had the focus as the hold began, taken before the modal's fields are disabled (Chrome moves the
  // focus off a disabled field to the body), to be given back once it is let go
  focus: RefObject<Element | null>;
  // whileHeld holds the modal while write and the read after it go, and lets go however it ends; the caller closes
  // the modal after
  whileHeld: <T>(write: () => Promise<T>) => Promise<T>;
}

// useHold is the hold of one modal while it writes (Dialog's held)
// LIMIT: a write that never answers (the store's lock held by a stopped process) holds the modal till the page is
// loaded again. Let a write be called off if that comes to trouble someone
export function useHold(): Hold {
  const [held, setHeld] = useState(false);
  const focus = useRef<Element | null>(null);
  const whileHeld = useCallback(async <T>(write: () => Promise<T>): Promise<T> => {
    focus.current = document.activeElement;
    setHeld(true);
    try {
      return await write();
    } finally {
      setHeld(false);
    }
  }, []);
  return { held, focus, whileHeld };
}
