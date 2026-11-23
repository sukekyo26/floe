import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ScreenSwitch } from '@/components/screen-switch';
import { testBoot } from '@/components/test-boot';
import type { Boot } from '@/config/boot';
import { createListQuery } from '@/stores/list-query';

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
const draw = (tasks: boolean) => {
  testBoot();
  return renderToString(<ScreenSwitch tasks={tasks} query={query('?project=p&view=todo')} />);
};

describe(ScreenSwitch, () => {
  it('is a nav of two links to the lists the tab last had, the screen shown the current page, and no tab', () => {
    expect.assertions(4);
    expect(draw(true)).toMatch(/^<nav id="screens" aria-label="ui_screens:/);
    expect(draw(true)).toMatch(
      /<a id="to-list" href="\/"(?![^>]*aria-current)[^>]*>.*<a id="to-tasks" href="\/tasks\?[^"]*project=p[^"]*" aria-current="page"/,
    );
    expect(draw(false)).toMatch(
      /<a id="to-list" href="\/" aria-current="page".*<a id="to-tasks" href="\/tasks\?[^"]*"(?![^>]*aria-current)/,
    );
    expect(draw(true)).not.toContain('role="tab');
  });

  it('sends the task list’s link where a task’s page says, its press with it', () => {
    expect.assertions(1);
    testBoot();
    expect(
      renderToString(
        <ScreenSwitch
          tasks
          taskUp={{ href: '/tasks?project=p&view=done', onPress: () => undefined }}
          query={query('')}
        />,
      ),
    ).toMatch(/<a id="to-tasks" href="\/tasks\?project=p&amp;view=done" aria-current="page"/);
  });

  it('draws both links of one weight, the screen shown darker, with no line under them', () => {
    expect.assertions(3);
    const html = draw(true);
    expect(html).toMatch(/id="to-list"[^>]*class="[^"]*font-semibold[^"]*text-muted[^"]*"/);
    expect(html).toMatch(/id="to-tasks"[^>]*class="[^"]*font-semibold[^"]*text-fg[^"]*"/);
    expect(html).not.toMatch(/<nav id="screens".*border-[bt]/s);
  });
});
