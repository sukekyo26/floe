// A value that tells those who read it when it changes. Its three calls are all the stores use, so that another
// store library could take its place here alone.
export interface Store<T> {
  get: () => T;
  set: (value: T) => void;
  subscribe: (listener: () => void) => () => void;
}

// createStore holds value; a set of the same value (Object.is) tells no one
export function createStore<T>(value: T): Store<T> {
  const listeners = new Set<() => void>();
  return {
    get: () => value,
    set: (next) => {
      if (Object.is(next, value)) return;
      value = next;
      for (const listener of [...listeners]) listener();
    },
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}
