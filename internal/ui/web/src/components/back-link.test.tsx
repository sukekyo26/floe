import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { BackLink } from '@/components/back-link';
import { testBoot } from '@/components/test-boot';
import { backOf } from '@/utils/back';

const draw = (kept: string) => {
  testBoot();
  return renderToString(<BackLink back={backOf(kept)} onPress={() => undefined} />);
};

describe(BackLink, () => {
  it("goes back to the task's page it was opened from, by the task's name, a plain link", () => {
    expect.assertions(3);
    const html = draw('/tasks?project=p&task=%23128');
    expect(html).toMatch(/^<a id="back" href="\/tasks\?project=p&amp;task=%23128" aria-label="ui_back_to_task:#128"/);
    expect(html).toMatch(/<span id="back-word"[^>]*>#128<\/span>/);
    expect(html).not.toMatch(/hover:bg-|rounded/);
  });

  it('is not drawn opened from either list, whose header link is the way back', () => {
    expect.assertions(2);
    expect(draw('/?view=fav')).toBe('');
    expect(draw('/tasks?project=p')).toBe('');
  });
});
