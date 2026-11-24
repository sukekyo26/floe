import { useSyncExternalStore } from 'react';

const MINUTE_MS = 60_000;

export interface Minute {
  get: () => number;
  subscribe: (listener: () => void) => () => void;
}

// createMinute is the time now to the minute, told every minute while one reads it: the clock runs from the first to the
// last that listens. A value to the minute stays the same between reads, as React asks of it, without being kept
export function createMinute(now: () => number = () => Date.now()): Minute {
  const listeners = new Set<() => void>();
  let timer: ReturnType<typeof setInterval> | null = null;
  return {
    get: () => Math.floor(now() / MINUTE_MS) * MINUTE_MS,
    subscribe: (listener) => {
      listeners.add(listener);
      timer ??= setInterval(() => {
        for (const l of [...listeners]) l();
      }, MINUTE_MS);
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0 && timer !== null) {
          clearInterval(timer);
          timer = null;
        }
      };
    },
  };
}

const minute = createMinute();

// useMinute draws anew every minute, for the times "3 minutes ago" and the days' headings
export const useMinute = (clock: Minute = minute): number =>
  useSyncExternalStore(clock.subscribe, clock.get, clock.get);
