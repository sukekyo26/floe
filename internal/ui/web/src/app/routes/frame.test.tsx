import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { FrameRoute } from '@/app/routes/frame';
import { testBoot } from '@/components/test-boot';
import { getBoot, setBoot } from '@/config/boot';
import { pages } from '@/stores/pages';
import type { Page } from '@/types/page';

const page: Page = {
  slug: 'p',
  title: 'Notes',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  versionCount: 2,
};

// The frame of p, at search, as the list read has it (entries), with what the tab kept (sessionStorage)
function draw(search: string, entries: Page[] = [page], kept: Record<string, string> = {}) {
  testBoot();
  setBoot({ ...getBoot(), screen: 'frame', framed: 'p' });
  const held = new Map(Object.entries(kept));
  vi.stubGlobal('window', {
    location: {
      host: '127.0.0.1:7325',
      origin: 'http://127.0.0.1:7325',
      hash: '',
      search,
      href: `http://127.0.0.1:7325/a/p/${search}`,
    },
    sessionStorage: {
      getItem: (key: string) => held.get(key) ?? null,
      setItem: () => undefined,
      removeItem: () => undefined,
    },
  });
  pages.state.set({ ...pages.state.get(), read: true, entries });
  return renderToString(<FrameRoute />);
}

const shown = (html: string, id: string) => new RegExp(`id="${id}"(?![^>]*hidden="")`).test(html);

describe(FrameRoute, () => {
  it('shows the page found, and not the words of none found, its search or the diff', () => {
    expect.assertions(4);
    const html = draw('');
    expect(shown(html, 'frame')).toBe(true);
    expect(shown(html, 'notfound')).toBe(false);
    expect(shown(html, 'search')).toBe(false);
    expect(shown(html, 'diff')).toBe(false);
  });

  it('shows the words of a page not found with the list’s search to look for it, and no page', () => {
    expect.assertions(3);
    const html = draw('', []);
    expect(shown(html, 'notfound')).toBe(true);
    expect(shown(html, 'search')).toBe(true);
    expect(shown(html, 'frame')).toBe(false);
  });

  it('shows the diff in a diff, and neither the page, loading nothing, nor its tags', () => {
    expect.assertions(3);
    const html = draw('?diff');
    expect(shown(html, 'diff')).toBe(true);
    expect(html).toMatch(/<iframe id="frame"(?![^>]*src=)[^>]*hidden=""/);
    expect(shown(html, 'frame-tags')).toBe(false);
  });

  it('opened from a task’s page, goes back to it by its back link, which the help names closed', () => {
    expect.assertions(5);
    const back = '/tasks?project=x&task=%23128';
    const html = draw('', [page], { 'pages.order': JSON.stringify({ order: ['p'], shown: ['p'], back }) });
    expect(html).toMatch(
      /<a id="back" href="\/tasks\?project=x&amp;task=%23128"[^>]*>.*#128<\/span><\/a><nav id="screens"/s,
    );
    // Shown on any screen: no link of the header goes to a task's page
    expect(html).toMatch(/<a id="back"[^>]*class="inline-flex /);
    expect(html).toMatch(/<a id="to-tasks"[^>]*aria-current="page"/);
    expect(html).not.toMatch(/<a id="to-list"[^>]*aria-current/);
    expect(html).toContain('<dd id="keys-back">ui_back_to_task:#128</dd>');
  });

  it('opened from the page list, goes back to it by the header’s Pages, its back link there hidden till 40rem', () => {
    expect.assertions(3);
    const html = draw('', [page], {
      'pages.order': JSON.stringify({ order: ['p'], shown: ['p'], back: '/?view=fav' }),
    });
    expect(html).toMatch(/<a id="to-list" href="\/\?view=fav"/);
    expect(html).toMatch(/<a id="to-list"[^>]*aria-current="page"/);
    expect(html).toMatch(/<a id="back" href="\/\?view=fav"[^>]*class="hidden narrow:inline-flex /);
  });

  it('opened from the task list, goes back to it by the header’s Task list, its back link there hidden till 40rem', () => {
    expect.assertions(3);
    const html = draw('', [page], {
      'pages.order': JSON.stringify({ order: ['p'], shown: ['p'], back: '/tasks?project=x' }),
    });
    expect(html).toMatch(/<a id="to-tasks" href="\/tasks\?project=x"/);
    expect(html).toMatch(/<a id="to-tasks"[^>]*aria-current="page"/);
    expect(html).toMatch(/<a id="back" href="\/tasks\?project=x"[^>]*class="hidden narrow:inline-flex /);
  });
});
