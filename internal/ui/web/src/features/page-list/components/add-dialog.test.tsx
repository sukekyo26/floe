import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { AddDialog } from '@/features/page-list/components/add-dialog';
import { createAddLink } from '@/features/page-list/stores/add-link';

describe(AddDialog, () => {
  it('draws the fields and buttons by the ids the screen has, the path required, the values as kept', () => {
    expect.assertions(3);
    testBoot();
    const add = createAddLink();
    add.values.set({ path: '/kept.html', slug: '', description: '' });
    const html = renderToString(<AddDialog open={false} onClosed={() => undefined} add={add} />);
    expect(
      [
        'add-dialog',
        'add',
        'add-title',
        'add-path',
        'add-slug',
        'add-desc',
        'add-error',
        'add-cancel',
        'add-submit',
      ].filter((id) => !html.includes(`id="${id}"`)),
    ).toStrictEqual([]);
    expect(html).toMatch(/id="add-path"[^>]*required=""[^>]*data-autofocus=""[^>]*value="\/kept.html"/);
    expect(html).toMatch(/id="add-cancel"[^>]*data-close=""|data-close=""[^>]*id="add-cancel"/);
  });
});
