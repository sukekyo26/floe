import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { AddPageDialog } from '@/features/tasks/components/add-page-dialog';
import type { TaskOf } from '@/types/task';

const task: TaskOf = {
  project: 'p',
  name: '#1',
  title: 'Title',
  state: 'doing',
  at: '',
  rev: 1,
  categories: ['plan', 'review'],
  pages: [],
  archived: [],
  children: [],
};

const draw = () =>
  renderToString(
    <AddPageDialog
      open={{ task, back: () => true }}
      onClosed={() => undefined}
      writes={{ tasksOf: () => [task], putPageOn: () => Promise.reject(new Error('not sent')) }}
    />,
  );

describe(AddPageDialog, () => {
  it('is titled Add a page and chooses the category in it: none, chosen as it opens, then the task’s in their order', () => {
    expect.assertions(2);
    testBoot();
    const html = draw();
    expect(html).toMatch(/<h2 id="add-page-title"[^>]*>ui_page_add:undefined<\/h2>/);
    expect(html).toMatch(
      /ui_task_category:undefined<select id="add-page-category"[^>]*><option value="" selected="">ui_category_none:undefined<\/option><option value="plan">plan<\/option><option value="review">review<\/option><\/select>.*id="add-page-q"/s,
    );
  });
});
