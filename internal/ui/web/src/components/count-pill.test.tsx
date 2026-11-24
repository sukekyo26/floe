import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { CountPill } from '@/components/count-pill';

describe(CountPill, () => {
  it('is light digits on a dark pill', () => {
    expect.assertions(1);
    expect(renderToString(<CountPill n={2} />)).toMatch(
      /^<span data-testid="count" class="[^"]*\bbg-muted\b[^"]*\btext-on-ink\b[^"]*">2<\/span>$/,
    );
  });
});
