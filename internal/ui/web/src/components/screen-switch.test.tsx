import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ScreenSwitch, type Up } from '@/components/screen-switch';
import { testBoot } from '@/components/test-boot';
import type { Boot } from '@/config/boot';
import { createListQuery } from '@/stores/list-query';
import type { Screen } from '@/types/screen';
import type { Back } from '@/utils/back';

// A list query of a tab that kept the task list it was last on, the page list none
const query = (search: string) => {
  const held = new Map<string, string>();
  const made = createListQuery({
    boot: () => ({ screen: 'tasks', states: [{ name: 'doing', new: false, first: true }] }) as unknown as Boot,
    search: () => search,
    storage: {
      read: (key) => held.get(key) ?? null,
      write: (key, value) => {
        held.set(key, value);
      },
      remove: (key) => {
        held.delete(key);
      },
    },
    replace: () => undefined,
    href: () => `http://x/tasks${search}`,
    folders: { takeOpen: () => undefined },
  });
  made.sync();
  return made;
};
const draw = (screen: Screen, ups: { from?: Back['to']; listUp?: Up; taskUp?: Up } = {}) => {
  testBoot();
  return renderToString(<ScreenSwitch screen={screen} {...ups} query={query('?project=p&view=todo')} />);
};
const up = (href: string): Up => ({ href, onPress: () => undefined });

describe(ScreenSwitch, () => {
  it('is a nav of two links to the lists the tab last had, the screen shown the current page, and no tab', () => {
    expect.assertions(4);
    expect(draw('taskList')).toMatch(/^<nav id="screens" aria-label="ui_screens:/);
    expect(draw('taskList')).toMatch(
      /<a id="to-list" href="\/"(?![^>]*aria-current)[^>]*>.*<a id="to-tasks" href="\/tasks\?[^"]*project=p[^"]*" aria-current="page"/,
    );
    expect(draw('list')).toMatch(
      /<a id="to-list" href="\/" aria-current="page".*<a id="to-tasks" href="\/tasks\?[^"]*"(?![^>]*aria-current)/,
    );
    expect(draw('taskList')).not.toContain('role="tab');
  });

  it('sends the task list’s link where a task’s page says, its press with it', () => {
    expect.assertions(1);
    expect(draw('taskPage', { taskUp: up('/tasks?project=p&view=done') })).toMatch(
      /<a id="to-tasks" href="\/tasks\?project=p&amp;view=done" aria-current="page"/,
    );
  });

  it('marks the list a page whole was opened from, gone at 56rem or less, and sends the one it was opened from where its up says', () => {
    expect.assertions(5);
    expect(draw('frame', { from: 'pages' })).toMatch(/^<nav id="screens"[^>]*class="[^"]*mid:hidden/);
    expect(draw('list')).not.toContain('mid:hidden');
    expect(draw('frame', { from: 'pages', listUp: up('/?view=fav') })).toMatch(
      /<a id="to-list" href="\/\?view=fav" aria-current="page".*<a id="to-tasks" href="\/tasks\?[^"]*project=p[^"]*"(?![^>]*aria-current)/,
    );
    expect(draw('frame', { from: 'tasks', taskUp: up('/tasks?view=done') })).toMatch(
      /<a id="to-list" href="\/"(?![^>]*aria-current).*<a id="to-tasks" href="\/tasks\?view=done" aria-current="page"/,
    );
    expect(draw('frame', { from: 'task' })).toMatch(/<a id="to-tasks"[^>]*aria-current="page"/);
  });

  it('draws the screen shown alone on the thumb, the other muted on the track', () => {
    expect.assertions(4);
    const html = draw('taskList');
    expect(html).toMatch(/<nav id="screens"[^>]*class="[^"]*\bbg-line\b/);
    expect(html).toMatch(/id="to-list"[^>]*class="[^"]*font-semibold[^"]*text-muted[^"]*"/);
    expect(html).not.toMatch(/id="to-list"[^>]*class="[^"]*(bg-frame-bg|shadow)/);
    expect(html).toMatch(/id="to-tasks"[^>]*class="[^"]*font-semibold[^"]*bg-frame-bg[^"]*text-fg[^"]*shadow-sm/);
  });
});
