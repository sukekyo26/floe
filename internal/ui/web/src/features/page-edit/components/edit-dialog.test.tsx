import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { EditDialog, editNow } from '@/features/page-edit/components/edit-dialog';
import type { Page } from '@/types/page';

const page: Page = {
  slug: 'a',
  title: 'A',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  path: '/tmp/a.html',
};

describe(EditDialog, () => {
  it('fills the fields of the page, folds the slug and the project, and holds its error line hidden', () => {
    expect.assertions(4);
    testBoot();
    const html = renderToString(<EditDialog target={{ page, back: () => false }} onClosed={() => undefined} />);
    expect(html).toMatch(/id="edit-target"[^>]*>\/tmp\/a.html</);
    expect(html).toMatch(/<input[^>]*id="edit-name"[^>]*placeholder="ui_title_from_file:undefined"[^>]*value="A"/);
    expect(html).toMatch(/<details id="edit-more"(?![^>]*open)/);
    expect(html).toMatch(/<p id="edit-error"[^>]*hidden=""/);
  });

  it('tells the folded slug and project as typed', () => {
    expect.assertions(2);
    testBoot();
    expect(editNow({ title: '', slug: ' b ', project: '', description: '' })).toBe('b · ui_no_project:undefined');
    expect(editNow({ title: '', slug: 'b', project: 'p', description: '' })).toBe('b · p');
  });
});
