import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { createMoveAsk } from '@/features/tasks/stores/move-ask';
import type { Page } from '@/types/page';

const page = (slug: string, task?: Page['task']): Page => ({
  slug,
  title: slug.toUpperCase(),
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'p',
  ...(task ? { task } : {}),
});

describe(createMoveAsk, () => {
  it('asks nothing of a page on no task or on the one named, and yes at once', async () => {
    expect.assertions(3);
    testBoot();
    const ask = createMoveAsk({ entries: () => [page('a'), page('b', { name: 'S1' })] });
    await expect(ask.ask(page('a'), 'S1')).resolves.toBe(true);
    await expect(ask.ask(page('b'), ' S1 ')).resolves.toBe(true);
    expect(ask.asking.get()).toBeNull();
  });

  it('asks of the page as the list has it now, not as the modal opened, and answers once', async () => {
    expect.assertions(3);
    testBoot();
    // Opened on no task, the list has it on S2 since
    const ask = createMoveAsk({ entries: () => [page('a', { name: 'S2', category: 'design' })] });
    const asked = ask.ask(page('a'), 'S1');
    expect(ask.asking.get()?.lead).toBe('ui_task_move_lead:A');
    ask.answer(false);
    ask.answer(true);
    await expect(asked).resolves.toBe(false);
    expect(ask.asking.get()).toBeNull();
  });
});
