import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { createTaskPageActs } from '@/features/tasks/stores/task-page-acts';
import type { Client } from '@/lib/api-client';
import type { TasksAnswer, TasksChange } from '@/lib/tasks';
import type { Notice } from '@/stores/notice';
import type { Pages } from '@/stores/pages';
import { createPressOnce } from '@/stores/press-once';
import type { TaskWrites } from '@/stores/tasks';
import { createToasts } from '@/stores/toasts';
import type { Undo } from '@/stores/undo-toast';
import type { TaskOf } from '@/types/task';

const kid: TaskOf = { project: 'p', name: '#2', state: 'doing', at: '', rev: 4, pages: [], archived: [], children: [] };

// slow: the reads of the list never end
function made(answers: Partial<TasksAnswer>[], { deleted = 200, slow = false } = {}) {
  testBoot();
  const log: string[] = [];
  const undos: Undo[] = [];
  let release: () => void = () => undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const writes = {
    writeTask: async (_project: string, name: string, body: TasksChange) => {
      log.push(`write ${name} ${JSON.stringify(body)}`);
      const answer = answers.shift() ?? {};
      if (answer.status === 0) await held;
      if (answer.status === -1) throw new Error('offline');
      return { ok: true, status: 200, text: '', rev: 9, ...answer } as TasksAnswer;
    },
  } as unknown as TaskWrites;
  const pages = {
    load: () => {
      log.push('load');
      return slow ? new Promise<void>(() => undefined) : Promise.resolve();
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
    undoToast: (said, how, options) => {
      log.push(`undo ${said} ${String(options?.focus)}`);
      undos.push(how);
      return 1;
    },
    client,
    press: createPressOnce(),
    places: {
      kin: () => true,
      part: (up) => log.push(`part ${String(up)}`) > 0,
      inFamily: () => false,
    },
  });
  return { acts, log, release, undos };
}

describe(createTaskPageActs, () => {
  it('changes the state of the rev drawn, telling the rev written before the list is read again, then the words', async () => {
    expect.assertions(1);
    const { acts, log } = made([{ rev: 6 }]);
    await acts.changeState('p', '#1', 5, 'done', (rev) => log.push(`answered ${String(rev)}`));
    expect(log).toStrictEqual(['write #1 {"new":false,"rev":5,"state":"done"}', 'answered 6', 'load', 'say ']);
  });

  it('tells a refusal before the list is read again, but on a stale one, which the writes read', async () => {
    expect.assertions(1);
    const { acts, log } = made([
      { ok: false, status: 409, text: 'changed' },
      { ok: false, status: 400, text: 'no' },
      { status: -1 },
    ]);
    const answered = (rev: number | null) => log.push(`answered ${String(rev)}`);
    await acts.changeState('p', '#1', 5, 'done', answered);
    await acts.changeState('p', '#1', 5, 'done', answered);
    await acts.changeState('p', '#1', 5, 'done', answered);
    expect(log.filter((x) => !x.startsWith('write'))).toStrictEqual([
      'answered null',
      'say ui_state_failed:changed',
      'answered null',
      'load',
      'say ui_state_failed:no',
      'answered null',
      'load',
      'say ui_state_failed:offline',
    ]);
  });

  it('archives and reads again; refused, says so, reading again but on a stale refusal', async () => {
    expect.assertions(1);
    const { acts, log } = made([
      {},
      { ok: false, status: 409, text: 'changed' },
      { ok: false, status: 500, text: 'no' },
    ]);
    await acts.archive(1, kid, true);
    await acts.archive(1, kid, false);
    await acts.archive(1, kid, true);
    expect(log.filter((x) => !x.startsWith('write'))).toStrictEqual([
      'load',
      'say ui_save_failed:changed',
      'load',
      'say ui_save_failed:no',
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

  it('takes a child off, back and off again before the reads end, and archives and restores a task so', async () => {
    expect.assertions(1);
    const { acts, log, undos } = made([], { slow: true });
    const ended = new Promise((go) => setTimeout(go, 5));
    void acts.unlink(kid, '#1', '#1', false);
    await ended;
    await undos[0]?.undo();
    void acts.unlink(kid, '#1', '#1', false);
    await Promise.resolve();
    void acts.archive(1, kid, true);
    await new Promise((go) => setTimeout(go, 5));
    void acts.archive(1, kid, false);
    await new Promise((go) => setTimeout(go, 5));
    expect(log.filter((x) => x.startsWith('write'))).toStrictEqual([
      'write #2 {"new":false,"rev":4,"parent":""}',
      'write #2 {"new":false,"rev":9,"parent":"#1","parentNew":false}',
      'write #2 {"new":false,"rev":4,"parent":""}',
      'write #2 {"new":false,"rev":4,"state":"archived"}',
      'write #2 {"new":false,"rev":4,"state":"done"}',
    ]);
  });

  it('takes the parent back of the rev the write answered, and says a write that could not go', async () => {
    expect.assertions(2);
    const { acts, log, undos } = made([{ rev: 5 }, {}, { status: -1 }]);
    await acts.unlink(kid, '#1', '#1', true);
    await undos[0]?.undo();
    await acts.unlink(kid, '#1', '#1', true);
    expect(log.filter((x) => x.startsWith('write'))[1]).toBe(
      'write #2 {"new":false,"rev":5,"parent":"#1","parentNew":false}',
    );
    expect(log.at(-1)).toBe('say ui_save_failed:offline');
  });

  it('forgets a task gone already as removed, and tells why a refusal', async () => {
    expect.assertions(2);
    await expect(made([], { deleted: 404 }).acts.forget('p', '#1')).resolves.toBe('');
    await expect(made([], { deleted: 400 }).acts.forget('p', '#1')).resolves.toBe('it has a page');
  });
});
