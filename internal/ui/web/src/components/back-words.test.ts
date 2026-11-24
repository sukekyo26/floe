import { describe, expect, it } from 'vitest';

import { backWords } from '@/components/back-words';
import { testBoot } from '@/components/test-boot';
import { backOf } from '@/utils/back';

describe(backWords, () => {
  it('says in full where each way back goes', () => {
    expect.assertions(1);
    testBoot();
    expect(
      [
        backOf('/?view=new', 'http://127.0.0.1:7325'),
        backOf('/tasks?project=floe', 'http://127.0.0.1:7325'),
        backOf('/tasks?task=%23130', 'http://127.0.0.1:7325'),
      ].map(backWords),
    ).toStrictEqual(['ui_to_page_list:undefined', 'ui_tasks_back:undefined', 'ui_back_to_task:#130']);
  });
});
