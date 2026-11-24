import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { TaskRow } from '@/features/tasks/components/task-row';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';

const page = (slug: string, news?: string): Page => ({
  slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'p',
  ...(news ? { news } : {}),
});
const up: TaskOf = {
  project: 'p',
  name: '#1',
  title: 'Up',
  state: 'doing',
  at: '',
  rev: 1,
  pages: [],
  archived: [],
  children: [],
};
const task: TaskOf = {
  ...up,
  name: '#2',
  title: 'Two',
  at: '2026-11-21T00:00:00Z',
  urls: ['https://x.org/a/pull/9'],
  pages: [page('a', 'page'), page('b')],
  archived: [page('c')],
  parentTask: up,
};
const draw = (fresh = false) => {
  testBoot();
  return renderToString(<TaskRow task={task} index={0} kept={false} fresh={fresh} />);
};

describe(TaskRow, () => {
  it('is keyed by its task, links to its page, and says it unread and fresh by its attributes', () => {
    expect.assertions(3);
    expect(draw()).toMatch(/^<li data-key="task:\[&quot;p&quot;,&quot;#2&quot;\]" data-unread="true"/);
    expect(draw()).toContain('href="/tasks?project=p&amp;task=%232"');
    expect([draw().includes(' data-fresh='), draw(true).includes(' data-fresh="true"')]).toStrictEqual([false, true]);
  });

  it('says its pages, those archived, those unread, its links and its parent, with nothing to select or do', () => {
    expect.assertions(3);
    const html = draw();
    const testids = [...html.matchAll(/data-testid="([^"]+)"/g)].map((m) => m[1]);
    expect(testids).toStrictEqual([
      'lead',
      'dot',
      'title',
      'tname',
      'sub',
      'task-pages',
      'task-pages',
      'ver',
      'xlink',
      'kin',
      'tname',
      'kin-title',
    ]);
    expect(html).toContain('target="_blank" rel="noopener" title="https://x.org/a/pull/9"');
    expect(html).toContain('title="#2\nTwo\np\nhttps://x.org/a/pull/9"');
  });
});
