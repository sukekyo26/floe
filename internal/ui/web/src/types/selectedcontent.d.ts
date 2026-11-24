// The type import makes this file a module, so that the declaration adds to React's JSX elements, not replaces them
import type { DetailedHTMLProps, HTMLAttributes } from 'react';

declare module 'react' {
  namespace JSX {
    // <selectedcontent>, the copy of the chosen option in a select's button (customizable select), not in @types/react
    interface IntrinsicElements {
      selectedcontent: DetailedHTMLProps<HTMLAttributes<HTMLElement>, HTMLElement>;
    }
  }
}
