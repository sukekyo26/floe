import { useLayoutEffect, type RefObject } from 'react';

// Where a modal puts the page's toasts and error line while it is open: an empty box of its own for each, always there
export interface OverlayPlaces {
  toasts: RefObject<HTMLElement | null>;
  error?: RefObject<HTMLElement | null>;
}

// useMovedOverlay moves #toasts (and #error) into the modal while it is open: the top layer hides what is outside the
// modal. Each is moved by appendChild into the box given, and back into its own box ([data-home], always there) as the
// modal closes, so that again and again (StrictMode) it ends the same. A modal closing and another opening in one
// render go back then out: React runs every layout effect's cleanup of a render before any of its setups, so both
// modals move them in a layout effect
// LIMIT: a key or a press on a toast moved is the page's DOM's, not the modal's React tree's: the modal listens on the
// dialog element for what it must see of them
export function useMovedOverlay(open: boolean, places: OverlayPlaces): void {
  useLayoutEffect(() => {
    if (!open) return;
    const moved: string[] = [];
    for (const [id, place] of [
      ['toasts', places.toasts],
      ['error', places.error],
    ] as const) {
      const el = document.getElementById(id);
      if (!el || !place?.current) continue;
      place.current.append(el);
      moved.push(id);
    }
    return () => {
      for (const id of moved) {
        const el = document.getElementById(id);
        const home = document.querySelector(`[data-home="${id}"]`);
        if (el && home) home.append(el);
      }
    };
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- the places are refs of boxes always there
  }, [open]);
}
