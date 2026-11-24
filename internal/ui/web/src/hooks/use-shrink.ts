import { useLayoutEffect, type RefObject } from 'react';

// A theme's sample is drawn 900px wide
const SAMPLE_WIDTH = 900;

// shrinkOf is how much a sample is shrunk to a card width wide
export const shrinkOf = (width: number): number => width / SAMPLE_WIDTH;

// useShrink sets --shrink on every [data-testid=theme-shot] in box, by its width as it changes; drawn is what draws
// them anew (the cards read)
export function useShrink(box: RefObject<HTMLElement | null>, drawn: unknown): void {
  useLayoutEffect(() => {
    if (!box.current) return;
    const seen = new ResizeObserver((sizes) => {
      for (const size of sizes) {
        if (size.target instanceof HTMLElement)
          size.target.style.setProperty('--shrink', String(shrinkOf(size.contentRect.width)));
      }
    });
    for (const shot of box.current.querySelectorAll('[data-testid=theme-shot]')) seen.observe(shot);
    return () => {
      seen.disconnect();
    };
  }, [box, drawn]);
}
