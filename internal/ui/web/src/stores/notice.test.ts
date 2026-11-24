import { describe, expect, it } from 'vitest';

import { createNotice } from '@/stores/notice';

describe(createNotice, () => {
  it("keeps an action's mistake from a load's and a mark's, and lets each take back only its own", () => {
    expect.assertions(4);
    const { said, say } = createNotice();
    say('save failed');
    say('load failed', 'load');
    expect(said.get()).toStrictEqual({ message: 'save failed', from: '' });
    say('', 'load');
    expect(said.get()).toStrictEqual({ message: 'save failed', from: '' });
    say('');
    say('load failed', 'load');
    say('', 'read');
    expect(said.get()).toStrictEqual({ message: 'load failed', from: 'load' });
    say('save failed again');
    expect(said.get()).toStrictEqual({ message: 'save failed again', from: '' });
  });

  it('tells no one of the same mistake said again, as a load does every few seconds while the server is down', () => {
    expect.assertions(1);
    const { said, say } = createNotice();
    say('failed', 'load');
    let told = 0;
    said.subscribe(() => told++);
    say('failed', 'load');
    say('failed', 'load');
    expect(told).toBe(0);
  });
});
