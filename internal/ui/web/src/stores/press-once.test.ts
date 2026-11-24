import { describe, expect, it } from 'vitest';

import { createPressOnce } from '@/stores/press-once';

// gate is an act that ends as it is let go
function gate() {
  let go: (value: string) => void = () => undefined;
  const ended = new Promise<string>((resolve) => {
    go = resolve;
  });
  return {
    act: () => ended,
    go: () => {
      go('done');
    },
  };
}

describe(createPressOnce, () => {
  it('runs nothing for what is asked already, and runs it again once that is done', async () => {
    expect.assertions(4);
    const press = createPressOnce();
    const first = gate();
    const one = press('archive a', 'true', first.act);
    await expect(press('archive a', 'true', () => Promise.resolve('twice'))).resolves.toBeUndefined();
    await expect(press('archive b', 'true', () => Promise.resolve('another'))).resolves.toBe('another');
    first.go();
    await expect(one).resolves.toBe('done');
    await expect(press('archive a', 'true', () => Promise.resolve('again'))).resolves.toBe('again');
  });

  it('runs what asks something else, and the same as before once that was asked', async () => {
    expect.assertions(3);
    const press = createPressOnce();
    const first = gate();
    const second = gate();
    void press('archive a', 'true', first.act);
    void press('archive a', 'false', second.act);
    const third = gate();
    let ran = false;
    const last = press('archive a', 'true', () => {
      ran = true;
      return third.act();
    });
    expect(ran).toBe(true);
    // Those before end: what the last asked is asked still
    first.go();
    second.go();
    await Promise.resolve();
    await expect(press('archive a', 'true', () => Promise.resolve('twice'))).resolves.toBeUndefined();
    third.go();
    await expect(last).resolves.toBe('done');
  });

  it('lets go of what failed, thrown on', async () => {
    expect.assertions(2);
    const press = createPressOnce();
    await expect(press('a', 'x', () => Promise.reject(new Error('offline')))).rejects.toThrow('offline');
    await expect(press('a', 'x', () => Promise.resolve('again'))).resolves.toBe('again');
  });
});
