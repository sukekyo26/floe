// PressOnce runs act for a press, unless what it asks (to) of key (the action and what it is done to) is asked already,
// its act not done: then it runs nothing and answers undefined. One that asks something else runs, and is what is asked
// from then on. act is to last until what was pressed shows its result: a task's write, whose answer is drawn; a page's
// write and the read after it
export type PressOnce = <T>(key: string, to: string, act: () => Promise<T>) => Promise<T | undefined>;

export function createPressOnce(): PressOnce {
  const asked = new Map<string, { to: string }>();
  return async (key, to, act) => {
    if (asked.get(key)?.to === to) return undefined;
    const mine = { to };
    asked.set(key, mine);
    try {
      return await act();
    } finally {
      // Not what a press since asked
      if (asked.get(key) === mine) asked.delete(key);
    }
  };
}

// pressOnce is the page's one
export const pressOnce = createPressOnce();
