import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { createTaskPageActs } from '@/features/tasks/stores/task-page-acts';
import type { Client } from '@/lib/api-client';
import type { TasksAnswer, TasksChange } from '@/lib/tasks';
import type { Notice } from '@/stores/notice';
import type { Pages } from '@/stores/pages';
import type { TaskWrites } from '@/stores/tasks';
import { createToasts } from '@/stores/toasts';
import type { TaskOf } from '@/types/task';

const kid: TaskOf = { project: 'p', name: '#2', state: 'doing', at: '', rev: 4, pages: [], archived: [], children: [] };

function made(answers: Partial<TasksAnswer>[], deleted = 200) {
  testBoot();
  const log: string[] = [];
  let release: () => void = () => undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const writes = {
    writeTask: async (_project: string, name: string, body: TasksChange) => {
      log.push(`write ${name} ${JSON.stringify(body)}`);
      const answer = answers.shift() ?? {};
      if (answer.status === 0) await held;
      return { ok: true, status: 200, text: '', rev: 9, ...answer } as TasksAnswer;
    },
  } as unknown as TaskWrites;
  const pages = {
    load: () => {
      log.push('load');
      return Promise.resolve();
    },
    readRev: () => 4,
  } as unknown as Pages;
  const notice = { say: (m: string) => log.push(`say ${m}`) } as unknown as Notice;
  const client = {
    api: () => Promise.resolve(new Response('it has a page', { status: deleted })),
  } as unknown as Client;
  const acts = createTaskPageActs({
    writes,
    pages,
    notice,
    toasts: createToasts(),
    undoToast: (said, _how, options) => {
      log.push(`undo ${said} ${String(options?.focus)}`);
      return 1;
    },
    client,
    places: {
      kin: () => true,
      part: (up) => log.push(`part ${String(up)}`) > 0,
      inFamily: () => false,
    },
  });
  return { acts, log, release };
}

describe(createTaskPageActs, () => {
  it('changes the state of the rev drawn, reads again and takes the words away, answering the rev written', async () => {
    expect.assertions(2);
    const { acts, log } = made([{ rev: 6 }]);
    await expect(acts.changeState('p', '#1', 5, 'done')).resolves.toBe(6);
    expect(log).toStrictEqual(['write #1 {"new":false,"rev":5,"state":"done"}', 'load', 'say ']);
  });

  it('says a refusal, reading again but what the writes read on a stale one', async () => {
    expect.assertions(3);
    const { acts, log } = made([
      { ok: false, status: 409, text: 'changed' },
      { ok: false, status: 400, text: 'no' },
    ]);
    await expect(acts.changeState('p', '#1', 5, 'done')).resolves.toBeNull();
    await expect(acts.changeState('p', '#1', 5, 'done')).resolves.toBeNull();
    expect(log.filter((x) => !x.startsWith('write'))).toStrictEqual([
      'say ui_state_failed:changed',
      'load',
      'say ui_state_failed:no',
    ]);
  });

  it('archives once: not on the second press of a double one, nor while one is sent', async () => {
    expect.assertions(1);
    const { acts, log, release } = made([{ status: 0 }]);
    await acts.archive(2, kid, true);
    const first = acts.archive(1, kid, true);
    await acts.archive(1, kid, true);
    release();
    await first;
    expect(log).toStrictEqual(['write #2 {"new":false,"rev":4,"state":"archived"}', 'load']);
  });

  it('takes a child off once, with the way back, and the part’s button once read with no focus on the card', async () => {
    expect.assertions(1);
    const { acts, log, release } = made([{ status: 0, rev: 5 }]);
    const first = acts.unlink(kid, '#1', '#1', false);
    await acts.unlink(kid, '#1', '#1', false);
    release();
    await first;
    expect(log).toStrictEqual([
      'write #2 {"new":false,"rev":4,"parent":""}',
      'undo ui_task_unlinked:#2 false',
      'load',
      'part false',
    ]);
  });

  it('forgets a task gone already as removed, and tells why a refusal', async () => {
    expect.assertions(2);
    await expect(made([], 404).acts.forget('p', '#1')).resolves.toBe('');
    await expect(made([], 400).acts.forget('p', '#1')).resolves.toBe('it has a page');
  });
});
