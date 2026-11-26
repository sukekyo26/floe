import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { App } from '@/app/app';
import { testBoot } from '@/components/test-boot';
import { getBoot, setBoot } from '@/config/boot';

const draw = (search: string, screen: 'list' | 'tasks' = 'tasks') => {
  testBoot();
  // The server's state reads the address bar's host as it draws: node has no window
  vi.stubGlobal('window', { location: { host: '127.0.0.1:7325' } });
  setBoot({ ...getBoot(), screen });
  return renderToString(<App search={search} />);
};
const dialogs = (html: string) => [...html.matchAll(/<dialog[^>]* id="([^"]+)"/g)].map((m) => m[1]).toSorted();
const TASK_PAGE = [
  'add-category-dialog',
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
  it('draws the task list or a task page under the bar, a task page’s back link and the bar’s task list going one way', () => {
    expect.assertions(5);
    const list = draw('?project=p');
    const page = draw('?project=p&task=%231');
    expect(list).toMatch(/data-testid="bar".*<a id="to-tasks" href="\/tasks" aria-current="page"/s);
    expect(page).toMatch(/<a id="to-tasks" href="\/tasks\?project=[^"]*" aria-current="page"/);
    const href = (id: string) => new RegExp(`<a id="${id}" href="([^"]*)"`).exec(page)?.[1];
    expect(href('task-up')).toBe(href('to-tasks'));
    expect(list).not.toContain('id="task-up"');
    expect(list + page).not.toMatch(/id="(search|back)"/);
  });

  it('holds every modal of the screen closed, the shelf’s filters on the task list alone', () => {
    expect.assertions(2);
    expect(dialogs(draw('?project=p&task=%231'))).toStrictEqual(TASK_PAGE);
    expect(dialogs(draw('?project=p'))).toStrictEqual([...TASK_PAGE, 'filter-dialog'].toSorted());
  });

  it('draws the page list under the bar with its search, the shelf, the head of the list and its modals', () => {
    expect.assertions(4);
    const list = draw('', 'list');
    expect(list).toMatch(
      /data-testid="bar".*id="screens".*id="search".*data-testid="bar-end".*id="shelf".*data-testid="lhead".*id="count".*id="bulk".*id="list"/s,
    );
    expect(list).toMatch(/<div class="order-1 [^"]*">.*id="list".*<div data-home="error"/s);
    expect(list).toMatch(/<a id="to-list" href="\/" aria-current="page"/);
    expect(dialogs(list)).toStrictEqual([
      'add-dialog',
      'edit-dialog',
      'filter-dialog',
      'find',
      'keys',
      'move-dialog',
      'set-dialog',
      'settings',
      'tag-dialog',
    ]);
  });

  it('puts the screen after the mistake’s words by its order, which stand later in the document', () => {
    expect.assertions(1);
    expect(draw('?project=p')).toMatch(/<div class="order-1 [^"]*">.*<div data-home="error"/s);
  });

  it('draws a page whole under its bar with nothing of the page before the list is read, and the modals of every screen', () => {
    expect.assertions(7);
    testBoot();
    vi.stubGlobal('window', {
      location: {
        host: '127.0.0.1:7325',
        origin: 'http://127.0.0.1:7325',
        hash: '',
        href: 'http://127.0.0.1:7325/a/p/',
      },
    });
    setBoot({ ...getBoot(), screen: 'frame', framed: 'p' });
    const frame = renderToString(<App search="" />);
    expect(frame).toMatch(/data-testid="bar".*id="screens".*id="frame-title"/s);
    expect(frame).toMatch(/<a id="to-list"[^>]*aria-current="page"/);
    for (const id of ['frame', 'diff', 'notfound']) expect(frame).toMatch(new RegExp(`id="${id}"[^>]*hidden=""`));
    expect(frame).toContain('<div class="contents print:hidden"><div data-home="toasts"');
    expect(dialogs(frame)).toStrictEqual([
      'edit-dialog',
      'find',
      'keys',
      'move-dialog',
      'set-dialog',
      'settings',
      'tag-dialog',
    ]);
  });
});
