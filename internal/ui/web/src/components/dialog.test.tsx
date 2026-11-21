import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { testBoot } from '@/components/test-boot';

const draw = (held: boolean) => {
  testBoot();
  return renderToString(
    <Dialog id="d" open onClosed={() => undefined} hold={{ held, focus: { current: null } }} labelledBy="d-title">
      <DialogHead icon={null} titleId="d-title" title="Title" />
      <DialogActions>
        <DialogClose />
      </DialogActions>
    </Dialog>,
  );
};

describe(Dialog, () => {
  it('takes close requests until held, and holds its head, body and foot in one fieldset', () => {
    expect.assertions(4);
    expect(draw(false)).toContain('closedby="closerequest"');
    const held = draw(true);
    expect(held).toContain('closedby="none"');
    // The close button is in the fieldset disabled: it is held too
    expect(held).toMatch(/<fieldset disabled="".*data-testid="modal-head".*data-close="".*<\/fieldset>/s);
    expect(held).toContain('aria-labelledby="d-title"');
  });
});

describe(DialogActions, () => {
  it('gives the buttons of a modal’s foot the foot’s size unless they name one, and leaves those outside as they are', () => {
    expect.assertions(1);
    testBoot();
    const sizes = (html: string) =>
      [...html.matchAll(/<button[^>]*class="([^"]*)"/g)].map((m) => String(m[1]).includes('h-9'));
    const html = renderToString(
      <>
        <Button>outside</Button>
        <DialogActions>
          <Button>hold</Button>
          <Button size="bar">named</Button>
        </DialogActions>
      </>,
    );
    expect(sizes(html)).toStrictEqual([false, true, false]);
  });
});
