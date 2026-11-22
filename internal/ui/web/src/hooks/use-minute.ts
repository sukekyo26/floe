import { useSyncExternalStore } from 'react';

const MINUTE_MS = 60_000;

export interface Minute {
  get: () => number;
  subscribe: (listener: () => void) => () => void;
}

// createMinute is the time now, told every minute while one reads it: the clock runs from the first to the last that
// listens
export function createMinute(now: () => number = Date.now): Minute {
  const listeners = new Set<() => void>();
  let at = now();
  let timer: ReturnType<typeof setInterval> | null = null;
  return {
    get: () => at,
    subscribe: (listener) => {
      listeners.add(listener);
      if (timer === null) {
        at = now();
        timer = setInterval(() => {
          at = now();
          for (const l of [...listeners]) l();
        }, MINUTE_MS);
      }
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

// useMinute draws anew every minute, for the times "3 minutes ago" and the days' headings (ui.html refreshTimes)
export const useMinute = (clock: Minute = minute): number =>
  useSyncExternalStore(clock.subscribe, clock.get, clock.get);
