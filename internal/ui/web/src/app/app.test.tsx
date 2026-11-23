import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { App } from '@/app/app';
import { testBoot } from '@/components/test-boot';
import { getBoot, setBoot } from '@/config/boot';

const draw = (search: string) => {
  testBoot();
  // The server's state reads the address bar's host as it draws: node has no window
  vi.stubGlobal('window', { location: { host: '127.0.0.1:7325' } });
  setBoot({ ...getBoot(), screen: 'tasks' });
  return renderToString(<App search={search} />);
};
const dialogs = (html: string) => [...html.matchAll(/<dialog[^>]* id="([^"]+)"/g)].map((m) => m[1]).toSorted();
const TASK_PAGE = [
  'add-page-dialog',
  'edit-add-dialog',
  'edit-dialog',
  'edit-task-dialog',
  'find',
  'keys',
  'kin-dialog',
  'move-dialog',
  'set-dialog',
  'settings',
  'tag-dialog',
];

describe(App, () => {
  it('draws the task list or a task page under the bar, the task list its shelf too', () => {
    expect.assertions(4);
    const list = draw('?project=p');
    const page = draw('?project=p&task=%231');
    expect(list).toMatch(/data-testid="bar".*<a id="to-tasks" href="\/tasks" aria-current="page"/s);
    expect(page).toContain('id="task-up"');
    expect(list).not.toContain('id="task-up"');
    expect(list + page).not.toMatch(/id="(search|back)"/);
  });

  it('holds every modal of the screen closed, the shelf’s filters on the task list alone', () => {
    expect.assertions(2);
    expect(dialogs(draw('?project=p&task=%231'))).toStrictEqual(TASK_PAGE);
    expect(dialogs(draw('?project=p'))).toStrictEqual([...TASK_PAGE, 'filter-dialog'].toSorted());
  });

  it('puts the screen after the mistake’s words by its order, which stand later in the document', () => {
    expect.assertions(1);
    expect(draw('?project=p')).toMatch(/<div class="order-1 [^"]*">.*<div data-home="error"/s);
  });
});
