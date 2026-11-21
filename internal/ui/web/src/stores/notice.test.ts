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
});
