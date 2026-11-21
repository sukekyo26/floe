import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { Button } from '@/components/button';

describe(Button, () => {
  it('is a plain button unless told to submit, in one size and one kind', () => {
    expect.assertions(4);
    expect(renderToString(<Button>Close</Button>)).toContain('type="button"');
    const foot = renderToString(
      <Button size="foot" kind="primary" closes>
        Save
      </Button>,
    );
    expect(foot).toMatch(/h-9(?![^"]*h-8)/);
    expect(foot).toContain('bg-ink');
    expect(foot).toContain('data-close=""');
  });
});
