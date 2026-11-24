import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { TaskListEmpty } from '@/features/tasks/components/task-list-empty';

const hidden = (html: string) =>
  ['tasks-none', 'tasks-other', 'tasks-failed', 'tasks-empty'].map((id) =>
    new RegExp(`id="${id}"[^>]*hidden=""`).test(html),
  );

describe(TaskListEmpty, () => {
  it('draws all its words, hidden but the one shown, as the tests watch each', () => {
    expect.assertions(3);
    testBoot();
    const draw = (p: { nothing?: boolean; failed?: boolean; none?: string | null; other?: boolean }) =>
      renderToString(
        <TaskListEmpty nothing={false} failed={false} none={null} other={false} onAdd={() => undefined} {...p} />,
      );
    expect(hidden(draw({}))).toStrictEqual([true, true, true, true]);
    expect(hidden(draw({ none: 'No tasks in x', other: true }))).toStrictEqual([false, false, true, true]);
    expect(hidden(draw({ nothing: true }))).toStrictEqual([true, true, true, false]);
  });
});
