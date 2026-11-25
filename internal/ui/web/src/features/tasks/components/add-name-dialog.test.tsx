import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { AddNameDialog } from '@/features/tasks/components/add-name-dialog';

const draw = (kind: 'url' | 'category', id = 'edit-add') =>
  renderToString(
    <AddNameDialog
      id={id}
      open={{ kind, back: () => true }}
      name="#12"
      refusal={() => ''}
      onAdd={() => undefined}
      onClosed={() => undefined}
    />,
  );

describe(AddNameDialog, () => {
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

  it('names its parts by its id, two standing at once', () => {
    expect.assertions(2);
    testBoot();
    const html = draw('category', 'other');
    expect([...html.matchAll(/ id="([^"]+)"/g)].map((m) => m[1])).toStrictEqual([
      'other-dialog',
      'other-title',
      'other-target',
      'other-new',
      'other-said',
      'other-submit',
    ]);
    expect(html).toMatch(/aria-labelledby="other-title".*aria-describedby="other-said"/s);
  });
});
