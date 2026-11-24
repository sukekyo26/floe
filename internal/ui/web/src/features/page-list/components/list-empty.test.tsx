import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { ListEmpty } from '@/features/page-list/components/list-empty';

const none = () => undefined;

describe(ListEmpty, () => {
  it('draws the way to add a link with the smaller plus of what opens an add', () => {
    expect.assertions(1);
    testBoot();
    const html = renderToString(
      <ListEmpty shown="empty" archived={0} onAdd={none} onShowArchived={none} onClear={none} />,
    );
    expect(/<button[^>]*id="add-empty"[^>]*class="[^"]*\[&amp;_svg\]:size-3.25 /.test(html)).toBe(true);
  });
});
