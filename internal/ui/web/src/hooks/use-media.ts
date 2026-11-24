import { useCallback, useSyncExternalStore } from 'react';

// useMedia tells whether the window meets query now, drawn anew as that changes; false where there is no window (drawn
// to a string)
export function useMedia(query: string): boolean {
  const listen = useCallback(
    (changed: () => void) => {
      const media = window.matchMedia(query);
      media.addEventListener('change', changed);
      return () => {
        media.removeEventListener('change', changed);
      };
    },
    [query],
  );
  return useSyncExternalStore(
    listen,
    () => window.matchMedia(query).matches,
    () => false,
  );
}
