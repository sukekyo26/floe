import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { BackLink } from '@/components/back-link';
import { testBoot } from '@/components/test-boot';
import { backOf } from '@/utils/back';

const draw = (kept: string) => {
  testBoot();
  return renderToString(<BackLink back={backOf(kept, 'http://127.0.0.1:7325')} onPress={() => undefined} />);
};

describe(BackLink, () => {
  it("goes back to the task's page it was opened from, by the task's name, a plain link", () => {
    expect.assertions(3);
    const html = draw('/tasks?project=p&task=%23128');
    expect(html).toMatch(/^<a id="back" href="\/tasks\?project=p&amp;task=%23128" aria-label="ui_back_to_task:#128"/);
    expect(html).toMatch(/<span id="back-word"[^>]*>#128<\/span>/);
    expect(html).not.toMatch(/hover:bg-|rounded/);
  });

  it('opened from either list, is there for h and narrow screens alone, hidden on a wider one, saying where it goes', () => {
    expect.assertions(2);
    expect(draw('/?view=fav')).toMatch(
      /^<a id="back" href="\/\?view=fav" aria-label="ui_to_page_list:undefined"[^>]*class="hidden narrow:inline-flex /,
    );
    expect(draw('/tasks?project=p')).toMatch(
      /aria-label="ui_tasks_back:undefined"[^>]*class="hidden narrow:inline-flex /,
    );
  });

  it('opened from a task’s page, shows on any screen', () => {
    expect.assertions(1);
    expect(draw('/tasks?project=p&task=%23128')).toMatch(/class="inline-flex /);
  });
});
