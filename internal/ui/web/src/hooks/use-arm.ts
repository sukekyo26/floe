import { useState } from 'react';

// armNext is a press of a button that acts on its second press: unarmed, it arms (and does nothing more); armed, it
// acts, and stays armed until its caller lets it go (a delete that failed is pressed again as it is)
export const armNext = (armed: boolean): { armed: true; act: boolean } => ({ armed: true, act: armed });

export interface Arm {
  armed: boolean;
  // press arms the button and answers true, or, armed, answers false: the caller acts
  press: () => boolean;
  // disarm lets it go: as the focus leaves it, and as its caller says (the selection changed, a delete failed)
  disarm: () => void;
}

// useArm is a button that asks again before it deletes (ui.html arm and disarm)
export function useArm(): Arm {
  const [armed, setArmed] = useState(false);
  return {
    armed,
    press: () => {
      const next = armNext(armed);
      setArmed(next.armed);
      return !next.act;
    },
    disarm: () => {
      setArmed(false);
    },
  };
}
