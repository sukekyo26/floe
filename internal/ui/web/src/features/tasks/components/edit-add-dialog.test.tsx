import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { EditAddDialog } from '@/features/tasks/components/edit-add-dialog';

const draw = (kind: 'url' | 'category') =>
  renderToString(
    <EditAddDialog
      open={{ kind, back: () => true }}
      name="#12"
      refusal={() => ''}
      onAdd={() => undefined}
      onClosed={() => undefined}
    />,
  );

describe(EditAddDialog, () => {
  it('adds a URL in a field of its own form, a refusal told under it, Cancel and Add', () => {
    expect.assertions(4);
    testBoot();
    const html = draw('url');
    expect(html).toMatch(/<h2 id="edit-add-title"[^>]*>ui_url_add:undefined<\/h2>/);
    expect(html).toMatch(
      /<form[^>]*>.*<input id="edit-add-new" data-testid="path" class="[^"]*font-mono"[^>]*spellCheck="false"[^>]*placeholder="https:\/\/" aria-describedby="edit-add-said"/s,
    );
    expect(html).toMatch(/<p id="edit-add-said" aria-live="polite"/);
    expect(html).toMatch(
      /ui_cancel:undefined<\/button>.*<button[^>]*type="submit"[^>]*id="edit-add-submit"[^>]*>ui_add:undefined/s,
    );
  });

  it('adds a category by its name, as text', () => {
    expect.assertions(2);
    testBoot();
    const html = draw('category');
    expect(html).toMatch(/<h2 id="edit-add-title"[^>]*>ui_category_add:undefined<\/h2>/);
    expect(html).toMatch(
      /ui_category_name:undefined<input id="edit-add-new" data-testid="text" class="[^"]*"(?![^>]*font-mono)/,
    );
  });
});
