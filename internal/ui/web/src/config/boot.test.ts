import { describe, expect, it } from 'vitest';

import { readBoot, screenOf } from '@/config/boot';

describe(readBoot, () => {
  it('reads the boot values, and refuses none and what is not JSON, saying to reload', () => {
    expect.assertions(4);
    expect(readBoot('{"screen":"list","framed":""}')).toStrictEqual({ screen: 'list', framed: '' });
    expect(() => readBoot(null)).toThrow('the page has no boot values: reload it');
    expect(() => readBoot('')).toThrow('the page has no boot values: reload it');
    expect(() => readBoot('{')).toThrow(/^the page has no boot values: reload it \(.+\)$/u);
  });
});

describe(screenOf, () => {
  it("tells a task's page from the task list by task= alone, and the list and a page whole by the server's screen", () => {
    expect.assertions(1);
    expect([
      screenOf({ screen: 'tasks' }, '?project=floe'),
      screenOf({ screen: 'tasks' }, '?project=floe&task=S1'),
      screenOf({ screen: 'list' }, '?task=S1'),
      screenOf({ screen: 'frame' }, '?task=S1'),
    ]).toStrictEqual(['taskList', 'taskPage', 'list', 'frame']);
  });
});
