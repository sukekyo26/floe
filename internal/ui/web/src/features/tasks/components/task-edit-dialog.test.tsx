import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { TaskEditDialog } from '@/features/tasks/components/task-edit-dialog';
import { createStore } from '@/lib/store';
import type { Pages, PagesState } from '@/stores/pages';
import type { TaskOf } from '@/types/task';

const task: TaskOf = {
  project: 'p',
  name: '#12',
  title: 'T',
  state: 'doing',
  at: '',
  rev: 3,
  urls: ['https://a', 'https://b'],
  pages: [],
  archived: [],
  children: [],
};
// The list as the modal reads it as it opens: the task, read at rev 3
const pages = {
  state: createStore<PagesState>({
    read: true,
    entries: [],
    tasks: { error: '', tasks: {}, raw: '' },
    readWhy: '',
    drawWhy: '',
  }),
  readRev: () => 3,
} as unknown as Pages;
const writes = { tasksOf: () => [task], writeTask: () => Promise.reject(new Error('no write')) };

describe(TaskEditDialog, () => {
  it('adds by a button under each list’s box, in a modal beside its own, the box hidden while empty', () => {
    expect.assertions(5);
    testBoot();
    const html = renderToString(
      <TaskEditDialog open={{ task, back: () => true }} onClosed={() => undefined} pages={pages} writes={writes} />,
    );
    expect([html.includes('edit-url-new'), html.includes('edit-category-new')]).toStrictEqual([false, false]);
    // The add modal first, beside the edit modal: neither its Escape nor its form reaches the edit modal's
    expect(html).toMatch(/^<dialog id="edit-add-dialog".*?<\/dialog><dialog id="edit-task-dialog"/s);
    // The URLs' box, then its button out of it
    expect(html).toMatch(
      /<div class="[^"]*border[^"]*"><ol id="edit-urls".*?<\/ol><\/div><span[^>]*><button[^>]*id="edit-url-plus"/s,
    );
    expect(html).toMatch(/<div class="[^"]*border[^"]*" hidden=""><ol id="edit-categories"/);
    expect(html).toMatch(/id="edit-category-plus"/);
  });
});
