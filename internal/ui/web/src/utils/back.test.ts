import { describe, expect, it } from 'vitest';

import { backOf } from '@/utils/back';

describe(backOf, () => {
  it('reads the way back to the page list, the task list and a task’s page', () => {
    expect.assertions(3);
    expect(backOf('/?view=new', 'http://127.0.0.1:7325')).toStrictEqual({ href: '/?view=new', to: 'pages', task: '' });
    expect(backOf('/tasks?project=floe', 'http://127.0.0.1:7325')).toStrictEqual({
      href: '/tasks?project=floe',
      to: 'tasks',
      task: '',
    });
    expect(backOf('/tasks?project=floe&task=%23130', 'http://127.0.0.1:7325')).toStrictEqual({
      href: '/tasks?project=floe&task=%23130',
      to: 'task',
      task: '#130',
    });
  });
});
