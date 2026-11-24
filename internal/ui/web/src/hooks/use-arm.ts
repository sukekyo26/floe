import { useState } from 'react';

// armNext is a press of a button that acts on its second press: unarmed, it arms (and does nothing more); armed, it
// acts, and stays armed until its caller lets it go (a page's delete that failed stays armed, to be pressed again)
// It is armed for one key (a page's slug): armed for another, it arms for this one, so a bar that draws the next page
// does not carry the press over to it
export const armNext = (armedFor: string | null, key: string): { armedFor: string; act: boolean } => ({
  armedFor: key,
  act: armedFor === key,
});

// armedAt is whether the button shows armed for key: armed for another page, it shows unarmed
export const armedAt = (armedFor: string | null, key: string): boolean => armedFor === key;

export interface Arm {
  armed: boolean;
  // press arms the button and answers true, or, armed, answers false: the caller acts
  press: () => boolean;
  // disarm lets it go: as the focus leaves it, and as its caller says (the selection changed, a task's delete failed)
  disarm: () => void;
}

// useArm is a button that asks again before it deletes
export function useArm(key: string): Arm {
  const [armedFor, setArmedFor] = useState<string | null>(null);
  return {
    armed: armedAt(armedFor, key),
    press: () => {
      const next = armNext(armedFor, key);
      setArmedFor(next.armedFor);
      return !next.act;
    },
    disarm: () => {
      setArmedFor(null);
    },
  };
}
