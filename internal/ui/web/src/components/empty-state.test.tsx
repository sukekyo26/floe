import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import built from '/dist/index.html?raw';

import { EmptyState } from '@/components/empty-state';

describe(EmptyState, () => {
  // Tailwind makes no rule for a class of a size the theme does not have (text-base, as --text-* is initial): its
  // words were left at the box's size. The build as committed is read, so a class added here fails this till the dist
  // is built again (just web-build)
  // LIMIT: reads the minified CSS's text for the escaped class and its font-size: a rule written another way (shared,
  // or reordered by the minifier) fails it though there. Read the CSS's rules if that comes
  it('has a rule in the build that sets its first words at 1rem', () => {
    expect.assertions(2);
    expect(
      renderToString(
        <EmptyState id="x" hidden={false}>
          {null}
        </EmptyState>,
      ),
    ).toContain('[&amp;_strong]:text-[1rem]');
    expect(built).toMatch(/\\\[\\&_strong\\\]\\:text-\\\[1rem\\\][^{]*\{font-size:1rem/);
  });
});
