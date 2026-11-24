import { useSyncExternalStore } from 'react';

const listen = (changed: () => void) => {
  window.addEventListener('resize', changed);
  return () => {
    window.removeEventListener('resize', changed);
  };
};

// useWindowWidth is the window's width, drawn anew as it changes; 0 where there is no window (drawn to a string)
export const useWindowWidth = (): number =>
  useSyncExternalStore(
    listen,
    () => window.innerWidth,
    () => 0,
  );

const rootRem = () => parseFloat(getComputedStyle(document.documentElement).fontSize);

// useRem is the root's font size in px, the browser's text, read anew as the window resizes; 16 drawn to a string
export const useRem = (): number => useSyncExternalStore(listen, rootRem, () => 16);
