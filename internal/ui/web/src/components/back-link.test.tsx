import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { BackLink, type BackLinkProps } from '@/components/back-link';

const up = { href: '/tasks?project=p', onPress: () => undefined };
const draw = (over: Partial<BackLinkProps> = {}) =>
  renderToString(<BackLink id="back" up={up} label="Back to #128" {...over} />);

describe(BackLink, () => {
  it('goes where its way up does, its word beside its mark, a plain link named and tipped in full', () => {
    expect.assertions(3);
    const html = draw({ word: '#128' });
    expect(html).toMatch(
      /^<a id="back" href="\/tasks\?project=p" aria-label="Back to #128" title="Back to #128"[^>]*class="inline-flex /,
    );
    expect(html).toMatch(/<span id="back-word"[^>]*>#128<\/span>/);
    expect(html).not.toMatch(/hover:bg-|rounded/);
  });

  it('of no word, is its mark alone', () => {
    expect.assertions(1);
    expect(draw()).toMatch(/<\/svg><\/a>$/);
  });

  it('shows only where a page whole’s bar has no switch, where told to', () => {
    expect.assertions(1);
    expect(draw({ narrowOnly: true })).toMatch(/class="hidden mid:inline-flex /);
  });
});
