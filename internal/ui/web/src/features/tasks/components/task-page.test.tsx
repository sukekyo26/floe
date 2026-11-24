import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { getBoot, setBoot } from '@/config/boot';
import { TaskFamily } from '@/features/tasks/components/task-family';
import { TaskHead } from '@/features/tasks/components/task-head';
import { TaskNone } from '@/features/tasks/components/task-none';
import { TaskPageRows } from '@/features/tasks/components/task-page-rows';
import { familyOf } from '@/features/tasks/utils/task-page';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';

const STATES = [
  { name: 'todo', new: true, first: false },
  { name: 'doing', new: false, first: true },
  { name: 'done', new: false, first: false },
  { name: 'archived', new: false, first: false },
];
const boot = () => {
  testBoot();
  setBoot({ ...getBoot(), screen: 'tasks', states: STATES });
};
const page = (slug: string, over: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '2026-11-21T00:00:00Z',
  taskProject: 'p',
  project: 'p',
  ...over,
});
const task = (name: string, over: Partial<TaskOf> = {}): TaskOf => ({
  project: 'p',
  name,
  title: 'Title',
  state: 'doing',
  at: '',
  rev: 1,
  pages: [],
  archived: [],
  children: [],
  ...over,
});
const none = () => undefined;
const openers = { onTags: none, onTheme: none, onEdit: none, onTaskSet: none };

describe(TaskFamily, () => {
  it('draws every part, hidden as it does not show, and its rows state first, the unread dot last', () => {
    expect.assertions(3);
    boot();
    const kid = task('#2', { state: 'done', parent: '#1', pages: [page('u', { news: 'version' })] });
    const of = task('#1', { children: [kid] });
    const html = renderToString(
      <TaskFamily task={of} family={familyOf(of, ['todo', 'doing', 'done'], 'en')} onKin={none} onUnlink={none} />,
    );
    expect(
      ['task-up-part', 'task-parent-add', 'task-parent-change', 'task-kids-part', 'task-child'].map((id) =>
        new RegExp(`id="${id}"[^>]*hidden=""`).test(html),
      ),
    ).toStrictEqual([true, false, true, false, false]);
    expect(html).toMatch(
      /data-testid="kin-row" href="\/tasks\?project=p&amp;task=%232" data-task="#2" data-unread="true" title="#2\nTitle" aria-label="ui_task_kin_row:#2 Title"[^>]*><span data-testid="kid-state"[^>]*>ui_state_done:undefined<\/span>.*data-testid="dot"[^>]*><\/span><\/a>/s,
    );
    expect(html).toMatch(/data-testid="unlink" data-task="#2" aria-label="ui_task_unlink_kid:#2"/);
  });
});

describe(TaskHead, () => {
  const head = (of: TaskOf) =>
    renderToString(
      <TaskHead
        task={of}
        shown
        project="p"
        name={of.name}
        reads={0}
        color="red"
        onKin={none}
        onEdit={none}
        onAddPage={none}
      />,
    );

  it('draws an archived task with the word for its choices and the way to give it back', () => {
    expect.assertions(3);
    boot();
    const html = head(task('#1', { state: 'archived' }));
    expect(html).toMatch(/<select[^>]*id="task-state"[^>]*hidden=""/);
    expect(html).toMatch(/id="task-archived"(?![^>]*hidden="")/);
    expect(html).toMatch(/id="task-archive"(?![^>]*hidden="").*ui_unarchive:/s);
  });

  it('draws the choices in the state’s colours, and Add a page by Edit, no way back of its own', () => {
    expect.assertions(4);
    boot();
    const html = head(task('#1', { state: 'done', urls: ['https://x.org/a'] }));
    expect(html).toMatch(/<select[^>]*id="task-state"[^>]*data-state="done"/);
    expect(html).not.toContain('id="task-up"');
    expect(html).toMatch(
      /<button[^>]*id="task-add-page"[^>]*>.*ui_page_add:undefined<\/button><button[^>]*id="task-edit"/s,
    );
    expect(html).toMatch(/id="task-link"(?![^>]*hidden="")[^>]*><a data-testid="xlink" href="https:\/\/x.org\/a"/);
  });
});

describe(TaskPageRows, () => {
  it('draws no box nor Select, tags as words, no way to the task, an archived page badged, headings with no adds', () => {
    expect.assertions(5);
    boot();
    const on = (slug: string, category: string, over: Partial<Page> = {}) =>
      page(slug, { task: { name: '#1', category }, ...over });
    const html = renderToString(
      <TaskPageRows
        task={task('#1', {
          categories: ['plan', 'empty'],
          pages: [on('a', 'plan', { tags: ['t'] })],
          archived: [on('b', 'plan', { archived: true })],
        })}
        openers={openers}
      />,
    );
    expect([html.includes('data-testid="sel"'), html.includes('data-testid="task-tag"')]).toStrictEqual([false, false]);
    expect(html).toMatch(/data-testid="select"[^>]*hidden=""/);
    expect(html).toMatch(/<span data-testid="tagpill" title="t"/);
    expect(html).toMatch(/data-slug="b"[^>]*data-archived="true"|data-archived="true"[^>]*data-slug="b"/);
    expect(html).toMatch(
      /data-key="group:empty"[^>]*><div role="heading" aria-level="2">empty<\/div>.*data-testid="category-empty"/s,
    );
  });

  it('heads no category first where it has pages, named in words of its own, and no heading a button', () => {
    expect.assertions(3);
    boot();
    const rows = (pages: Page[]) =>
      renderToString(<TaskPageRows task={task('#1', { categories: ['plan'], pages })} openers={openers} />);
    const keys = (html: string) => [...html.matchAll(/data-key="([^"]*)"/g)].map((m) => m[1]);
    expect(keys(rows([page('a', { task: { name: '#1', category: 'plan' } })]))).toStrictEqual([
      'group:plan',
      'entry:a',
    ]);
    const html = rows([page('a', { task: { name: '#1' } })]);
    expect(keys(html)).toStrictEqual(['group:', 'entry:a', 'group:plan']);
    expect(html).toMatch(
      /data-key="group:"[^>]*><div role="heading" aria-level="2">ui_category_none:undefined<\/div><\/li>(?!.*<button[^>]*data-testid="category-add")/s,
    );
  });
});

describe(TaskNone, () => {
  it('hides the way to remove a task with children, and its words, saying why', () => {
    expect.assertions(1);
    boot();
    const html = renderToString(
      <TaskNone
        task={task('#1', { children: [task('#2')] })}
        none
        missing={false}
        project="p"
        name="#1"
        listHref="/tasks"
      />,
    );
    expect(
      ['task-none', 'task-delete-box', 'task-delete-error', 'task-none-kids', 'task-missing'].map((id) =>
        new RegExp(`id="${id}"[^>]*hidden=""`).test(html),
      ),
    ).toStrictEqual([false, true, true, false, true]);
  });
});
