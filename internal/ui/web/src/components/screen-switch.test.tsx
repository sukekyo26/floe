import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ScreenSwitch } from '@/components/screen-switch';
import { testBoot } from '@/components/test-boot';

const draw = (tasks: boolean) => {
  testBoot();
  return renderToString(<ScreenSwitch tasks={tasks} />);
};

describe(ScreenSwitch, () => {
  it('is a nav of two links, the screen shown the current page, and no tab', () => {
    expect.assertions(4);
    expect(draw(true)).toMatch(/^<nav id="screens" aria-label="ui_screens:/);
    expect(draw(true)).toMatch(
      /<a id="to-list" href="\/"(?![^>]*aria-current)[^>]*>.*<a id="to-tasks" href="\/tasks" aria-current="page"/,
    );
    expect(draw(false)).toMatch(
      /<a id="to-list" href="\/" aria-current="page".*<a id="to-tasks" href="\/tasks"(?![^>]*aria-current)/,
    );
    expect(draw(true)).not.toContain('role="tab');
  });
});
