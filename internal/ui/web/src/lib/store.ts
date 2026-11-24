// A value that tells those who read it when it changes. Its three calls are all the stores use, so that another
// store library could take its place here alone.
export interface Store<T> {
  get: () => T;
  set: (value: T) => void;
  subscribe: (listener: () => void) => () => void;
}

// report tells of a listener's throw as the browser tells of an event listener's (reportError), else on the console
function report(error: unknown): void {
  if (typeof globalThis.reportError === 'function') globalThis.reportError(error);
  else console.error(error);
}

// createStore holds value; a set of the same value (Object.is) tells no one
export function createStore<T>(value: T): Store<T> {
  const listeners = new Set<() => void>();
  return {
    get: () => value,
    set: (next) => {
      if (Object.is(next, value)) return;
      value = next;
      // A listener's throw stops neither the listeners after it nor the screen's draw: it is reported, as the DOM's
      // dispatchEvent does
      for (const listener of [...listeners]) {
        try {
          listener();
        } catch (error) {
          report(error);
        }
      }
    },
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}
