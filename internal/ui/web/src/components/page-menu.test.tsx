import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { PageMenu, type Openers, type Place } from '@/components/page-menu';
import { testBoot } from '@/components/test-boot';
import type { Page } from '@/types/page';

const none = () => undefined;
const openers: Openers = { onTags: none, onTheme: none, onEdit: none, onTaskSet: none };
const page: Page = { slug: 'a', title: 'A', description: '', createdAt: '', updatedAt: '', taskProject: 'floe' };

// tagOf is the opening tag of the item testid, as drawn
const tagOf = (html: string, testid: string) =>
  String(new RegExp(`<[a-z]+[^>]*data-testid="${testid}"[^>]*>`).exec(html)?.[0]);
const draw = (
  a: Partial<Page> = {},
  place: Place = 'row',
  taskShown: { project: string; name: string } | null = null,
  onPick: (() => void) | null = () => undefined,
) => {
  testBoot();
  return renderToString(
    <PageMenu
      page={{ ...page, ...a }}
      at={{ slug: 'a' }}
      place={place}
      menuId="m"
      openers={openers}
      taskShown={taskShown}
      {...(onPick ? { onPick } : {})}
    />,
  );
};
const hides = (html: string, testid: string) => tagOf(html, testid).includes(' hidden=""');

describe(PageMenu, () => {
  it('hides what a page has none of', () => {
    expect.assertions(5);
    const plain = draw();
    expect(
      ['mark-unread', 'theme-item', 'pin-item', 'task-out', 'task-open', 'copy'].map((id) => hides(plain, id)),
    ).toStrictEqual([false, true, false, true, true, true]);
    const link = draw({ path: '/tmp/a.html', themed: true });
    expect(['mark-unread', 'theme-item', 'copy'].map((id) => hides(link, id))).toStrictEqual([true, false, false]);
    expect(['mark-unread', 'pin-item'].map((id) => hides(draw({ archived: true }), id))).toStrictEqual([true, true]);
    expect(['open', 'download'].map((id) => hides(draw({ folder: true }), id))).toStrictEqual([true, true]);
    // A field written false does not stop the later ones from hiding it
    expect(hides(draw({ folder: false, news: 'version' }), 'mark-unread')).toBe(true);
  });

  it('leaves out the way to the task whose page it is, and shows it of another', () => {
    expect.assertions(2);
    const onTask = { task: { name: '#1' } };
    expect(hides(draw(onTask, 'row', { project: 'floe', name: '#1' }), 'task-open')).toBe(true);
    expect(hides(draw(onTask, 'row', { project: 'floe', name: '#2' }), 'task-out')).toBe(false);
  });

  it('shows Select in a row’s menu alone, and hides by its CSS what the bar or the row shows, the frame folding its own in', () => {
    expect.assertions(4);
    expect([
      hides(draw({}, 'row'), 'select'),
      hides(draw({}, 'list'), 'select'),
      hides(draw({}, 'row', null, null), 'select'),
    ]).toStrictEqual([false, true, true]);
    const classOf = (html: string, id: string) => String(/class="([^"]*)"/.exec(tagOf(html, id))?.[1]).split(' ');
    expect(['fav-item', 'archive', 'open'].map((id) => classOf(draw({}, 'row'), id).includes('hidden'))).toStrictEqual([
      true,
      true,
      false,
    ]);
    expect(['open', 'download'].map((id) => classOf(draw({}, 'list'), id).includes('hidden'))).toStrictEqual([
      true,
      true,
    ]);
    expect(
      ['fav-item', 'archive', 'open', 'download'].map((id) =>
        classOf(draw({}, 'frame'), id).includes('narrow:inline-flex'),
      ),
    ).toStrictEqual([true, true, true, true]);
  });
});
